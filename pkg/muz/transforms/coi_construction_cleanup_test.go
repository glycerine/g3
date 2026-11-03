package transforms

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestConeConstructionCleanupPreservesNestedOwnersDuringNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var manager *ast.ASTManager
	var temporary *base.Rule
	var set *base.RuleSet
	var innerSteps, outerSteps []ConeModelStep
	var failure error
	cause := errors.New("cone construction interrupted")
	active, callbacks, preserved := false, 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{
		Budget: budget, SizeTBits: 64,
		Transport: func(f memory.Failure) error {
			if active {
				callbacks++
				if failure == nil {
					inner := closeConeConstruction(manager, cause, []*base.Rule{temporary}, []*base.RuleSet{set}, innerSteps)
					failure = closeConeConstruction(nil, inner, nil, nil, outerSteps)
				}
				var owner *coneConstructionFailure
				preserved = preserved && errors.As(failure, &owner)
				if owner != nil {
					for retry := 0; retry < 2; retry++ {
						preserved = preserved && errors.Is(owner.Cleanup(), ast.ErrASTPending)
					}
					var nested *coneConstructionFailure
					preserved = preserved && errors.As(owner.nested, &nested)
					if nested != nil {
						preserved = preserved && len(nested.rules) == 1 && nested.rules[0] == temporary && len(nested.sets) == 1 && nested.sets[0] == set
					}
				}
				for _, steps := range [][]ConeModelStep{innerSteps, outerSteps} {
					for _, step := range steps {
						for _, definition := range step {
							preserved = preserved && definition.Formula != (ast.ASTRef{})
						}
					}
				}
			}
			return f
		},
	})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("cone construction fixture leaked bytes", budget.Used())
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	rules, err := base.NewRuleManager(manager)
	must(err)
	defer func() { must(rules.Close()) }()
	name, err := symbols.Intern("pending-cone-construction")
	must(err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(predicate))
	head, err := manager.MkApp(predicate, nil)
	must(err)
	temporary, err = rules.NewRule(head, nil, nil, "temporary")
	must(err)
	defer func() { must(temporary.Close()) }()
	set, err = base.NewRuleSet(rules)
	must(err)
	defer func() { must(set.Close()) }()
	stored, err := rules.NewRule(head, nil, nil, "stored")
	must(err)
	must(set.Add(stored))
	must(stored.Close())
	truth, err := manager.MkTrue()
	must(err)
	for index := 0; index < 3; index++ {
		formula, e := manager.NewRef(truth)
		must(e)
		definition := ConeModelDefinition{Declaration: predicate, Formula: formula, manager: manager}
		if index < 2 {
			innerSteps = append(innerSteps, ConeModelStep{definition})
		} else {
			outerSteps = append(outerSteps, ConeModelStep{definition})
		}
	}
	defer func() { must(closeConeModelSteps(innerSteps)); must(closeConeModelSteps(outerSteps)) }()

	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || callbacks == 0 || !preserved {
		t.Fatal("cone construction owners changed during refusal", body, callbacks, preserved)
	}
	var owner memory.ConstructionFailure
	if !errors.As(failure, &owner) || !errors.Is(failure, cause) || !errors.Is(owner.CleanupCause(), ast.ErrASTPending) || owner.RuntimeOwner() != runtime {
		t.Fatal("cone construction did not transport cleanup owners", failure)
	}
	receipt, receiptText := owner.Unwrap(), owner.Error()
	must(owner.Cleanup())
	must(owner.Cleanup())
	if owner.Unwrap() != receipt || owner.Error() != receiptText || owner.CleanupCause() != nil || owner.RuntimeOwner() != runtime || !errors.Is(receipt, cause) || !errors.Is(receipt, ast.ErrASTPending) {
		t.Fatal("cone cleanup changed the original failure receipt")
	}
	if _, err = temporary.Head(); !errors.Is(err, base.ErrRuleOwner) {
		t.Fatal("temporary rule survived cleanup", err)
	}
	if _, err = set.Len(); !errors.Is(err, base.ErrRuleSetOwner) {
		t.Fatal("temporary rule set survived cleanup", err)
	}
	for _, steps := range [][]ConeModelStep{innerSteps, outerSteps} {
		for _, step := range steps {
			for _, definition := range step {
				if definition.Formula != (ast.ASTRef{}) {
					t.Fatal("model step survived cleanup")
				}
			}
		}
	}
}
