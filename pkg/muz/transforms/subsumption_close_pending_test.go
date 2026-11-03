package transforms

// Source: dl_mk_subsumption_checker.cpp total-relation model converter.
// Its Go formula owners must remain reachable when native cleanup is refused.

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

func TestSubsumptionClosePreservesModelStepsWhileManagerExecutes(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	plugin := NewSubsumptionCheckerPlugin(true, 1)
	active, callbacks, intact := false, 0, true
	var original ast.ASTRef
	var manager *ast.ASTManager
	var pendingSet *base.RuleSet
	var constructionErr error
	constructionCause := errors.New("subsumption intermediate failure")
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			for i := 0; i < 2; i++ {
				cause := plugin.Close()
				intact = intact && errors.Is(cause, ast.ErrASTPending) && !plugin.closed && len(plugin.modelSteps) == 1
				if len(plugin.modelSteps) == 1 {
					intact = intact && len(plugin.modelSteps[0]) == 2 && plugin.modelSteps[0][0].Formula == original && plugin.modelSteps[0][1].Formula != (ast.ASTRef{})
				}
			}
			if pendingSet != nil && constructionErr == nil {
				constructionErr = closeSubsumptionConstruction(manager, constructionCause, []*base.RuleSet{pendingSet}, nil)
			}
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	manager = m
	defer func() {
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("fixture leaked bytes", budget.Used())
		}
	}()
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	set, err := base.NewRuleSet(rules)
	must(err)
	defer func() { must(set.Close()) }()
	var total, output ast.ASTHandle
	for _, name := range []string{"pending-subsumption-total", "pending-subsumption-output"} {
		s, e := symbols.Intern(name)
		must(e)
		d, e := m.MkFuncDecl(s, nil, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		must(rules.RegisterPredicate(d))
		h, e := m.MkApp(d, nil)
		must(e)
		var tails []ast.ASTHandle
		if !total.IsNull() {
			body, e := m.MkApp(total, nil)
			must(e)
			tails = []ast.ASTHandle{body}
		}
		r, e := rules.NewRule(h, tails, nil, name)
		must(e)
		must(set.Add(r))
		must(r.Close())
		if total.IsNull() {
			total = d
		} else {
			output = d
			must(set.SetOutputPredicate(d))
		}
	}
	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("invalid fixture")
	}
	transformed, err := plugin.Transform(set)
	must(err)
	if transformed == nil {
		t.Fatal("missing transformed rules")
	}
	pendingSet = transformed
	defer func() {
		if pendingSet != nil {
			must(pendingSet.Close())
		}
	}()
	defer func() { must(plugin.Close()) }()
	if len(plugin.modelSteps) != 1 || len(plugin.modelSteps[0]) != 2 || plugin.modelSteps[0][0].Declaration != total || plugin.modelSteps[0][1].Declaration != output {
		t.Fatal("missing total-relation model owner")
	}
	original = plugin.modelSteps[0][0].Formula
	h, err := m.RefGet(original)
	must(err)
	if !m.IsTrue(h) {
		t.Fatal("total interpretation is not true")
	}
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(m.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("refused close lost model ownership", body, callbacks, intact)
	}
	var construction memory.ConstructionFailure
	if !errors.As(constructionErr, &construction) || !errors.Is(constructionErr, constructionCause) || !errors.Is(construction.CleanupCause(), ast.ErrASTPending) || construction.RuntimeOwner() != runtime {
		t.Fatal("refused intermediate cleanup lost its owner", constructionErr)
	}
	receipt, receiptText := construction.Unwrap(), construction.Error()
	must(construction.Cleanup())
	must(construction.Cleanup())
	if construction.Unwrap() != receipt || construction.Error() != receiptText || !errors.Is(construction.Unwrap(), constructionCause) || !errors.Is(construction.Unwrap(), ast.ErrASTPending) || construction.CleanupCause() != nil {
		t.Fatal("intermediate cleanup changed its failure receipt", construction.Error(), construction.CleanupCause())
	}
	pendingSet = nil
	if steps, cause := plugin.TakeModelSteps(); !errors.Is(cause, base.ErrRuleTransformerOwner) || steps != nil {
		t.Fatal("closing plugin transferred model ledger", cause)
	}
	if next, cause := plugin.Transform(set); !errors.Is(cause, base.ErrRuleTransformerOwner) || next != nil {
		t.Fatal("closing plugin accepted transformation", cause)
	}
	after, err := m.RefGet(original)
	must(err)
	if after != h {
		t.Fatal("refused close changed interpretation")
	}
	before := m.Stats().LiveReferences
	must(plugin.Close())
	must(plugin.Close())
	if !plugin.closed || len(plugin.modelSteps) != 0 || m.Stats().LiveReferences+2 != before {
		t.Fatal("retry did not release exactly the model owner")
	}
}
