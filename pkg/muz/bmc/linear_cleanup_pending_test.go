package bmc

// Source: dl_bmc_engine.cpp linear::get_model trace/proof owner cleanup.

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

func TestLinearCleanupPreservesSoleRuleAndProofOwnersWhileManagerExecutes(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var manager *ast.ASTManager
	var owned *base.Rule
	var proofRefs, childRefs []ast.ASTRef
	var ruleFailure, proofFailure error
	callbacks, intact := 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		callbacks++
		before := proofRefs[0]
		childFailure := disposeASTRefs(manager, childRefs, nil)
		ruleFailure = disposeRuleOwners(manager, []*base.Rule{owned}, childFailure)
		proofFailure = disposeASTRefs(manager, proofRefs, ruleFailure)
		_, ownerErr := owned.Head()
		intact = intact && errors.Is(ruleFailure, ast.ErrASTPending) && errors.Is(proofFailure, ast.ErrASTPending) && proofRefs[0] == before && childRefs[0] != (ast.ASTRef{}) && ownerErr == nil
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
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
		if used := budget.Used(); used != 0 {
			t.Fatal("linear cleanup fixture leaked bytes", used)
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	rules, err := base.NewRuleManager(manager)
	must(err)
	defer func() { must(rules.Close()) }()
	name, err := symbols.Intern("linear-cleanup-goal")
	must(err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(declaration))
	head, err := manager.MkApp(declaration, nil)
	must(err)
	rule, err := rules.NewRule(head, nil, nil, "linear-cleanup")
	must(err)
	owned, err = rule.Retain()
	must(err)
	// Leave owned as the only Rule owner so its refused close reaches actual
	// AST roots instead of merely decrementing the shared Go owner count.
	must(rule.Close())
	proofRef, err := manager.NewRef(head)
	must(err)
	proofRefs = []ast.ASTRef{proofRef}
	childRef, err := manager.NewRef(head)
	must(err)
	childRefs = []ast.ASTRef{childRef}

	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("refused linear cleanup lost owners", body, callbacks, intact)
	}
	for _, failure := range []error{ruleFailure, proofFailure} {
		var cleanup memory.ConstructionFailure
		if !errors.As(failure, &cleanup) {
			t.Fatal("linear cleanup lost retry owner", failure)
		}
		reported := cleanup.Error()
		must(cleanup.Cleanup())
		if cleanup.Error() == "" || cleanup.Error() != reported || cleanup.Unwrap() == nil {
			t.Fatal("linear cleanup lost stable cause")
		}
	}
	if proofRefs[0] != (ast.ASTRef{}) || childRefs[0] != (ast.ASTRef{}) {
		t.Fatal("linear proof retry left reference")
	}
	if _, err = owned.Head(); err == nil {
		t.Fatal("linear rule retry left owner open")
	}
}
