package transforms

// Source: dl_mk_unfold.cpp rule-unifier ownership during unfolding.
// Its Go resolver must remain reachable when native cleanup is refused.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func testRuleUnfoldClosePreservesResolverWhileManagerExecutes(t *testing.T, afterSimplifier bool) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var plugin *RuleUnfoldPlugin
	active, callbacks, intact := false, 0, true
	var original *RuleInlinerPlugin
	var originalLite *qe.Lite
	var originalCooperation *cooperate.Cooperation
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			for i := 0; i < 2; i++ {
				cause := plugin.Close()
				intact = intact && errors.Is(cause, ast.ErrASTPending) && !plugin.closed && plugin.resolver == original && original.lite == originalLite && original.cooperate == originalCooperation
				if afterSimplifier {
					intact = intact && original.simplifier == nil
				}
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
	parameters, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, parameters, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	defer func() { must(globals.Close()); must(catalog.Close()); must(parameters.Close()) }()
	name, err := symbols.Intern("pending-unfold-goal")
	must(err)
	declaration, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(declaration))
	head, err := m.MkApp(declaration, nil)
	must(err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	guard, err := m.MkEq(zero, zero)
	must(err)
	pn, err := symbols.Intern("pending-unfold-fact")
	must(err)
	pd, err := m.MkFuncDecl(pn, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(pd))
	ph, err := m.MkApp(pd, nil)
	must(err)
	fact, err := rules.NewRule(ph, nil, nil, "pending-fact")
	must(err)
	must(set.Add(fact))
	must(fact.Close())
	rule, err := rules.NewRule(head, []ast.ASTHandle{ph, guard}, nil, "pending-guard")
	must(err)
	must(set.Add(rule))
	must(rule.Close())
	must(set.SetOutputPredicate(declaration))
	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("invalid fixture")
	}
	plugin = NewRuleUnfoldPlugin(m, rules, parameters, globals, 1)
	defer func() {
		if !plugin.closed {
			must(plugin.Close())
		}
	}()
	transformed, err := plugin.Transform(set)
	must(err)
	if transformed == nil {
		t.Fatal("fixture did not unfold predicate tail")
	}
	rewritten, err := transformed.Rule(1)
	must(err)
	tails, err := rewritten.TailSize()
	must(err)
	if tails != 0 {
		t.Fatal("guard was not simplified", tails)
	}
	must(rewritten.Close())
	must(transformed.Close())
	original = plugin.resolver
	if original.simplifier == nil || original.lite == nil || original.cooperate == nil {
		t.Fatal("unfolding did not initialize normalization owners")
	}

	originalLite, originalCooperation = original.lite, original.cooperate
	if afterSimplifier {
		// Stage the completed earlier child explicitly; the later refusal is
		// real, but this does not simulate a failure inside that first child.
		must(original.simplifier.Close())
		original.simplifier = nil
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
		t.Fatal("refused close lost resolver ownership", body, callbacks, intact)
	}

	if next, cause := plugin.Transform(set); !errors.Is(cause, base.ErrRuleTransformerOwner) || next != nil {
		t.Fatal("closing plugin accepted transformation", cause)
	}

	must(plugin.Close())
	if cause := plugin.Close(); !errors.Is(cause, base.ErrRuleTransformerOwner) {
		t.Fatal("closed-owner guard changed", cause)
	}
	if !plugin.closed || plugin.resolver != nil || !original.closed || original.lite != nil || original.cooperate != nil {
		t.Fatal("retry did not release child resolver")
	}
}

func TestRuleUnfoldClosePreservesResolverWhileManagerExecutes(t *testing.T) {
	t.Run("initial", func(t *testing.T) {
		testRuleUnfoldClosePreservesResolverWhileManagerExecutes(t, false)
	})
	t.Run("after-simplifier", func(t *testing.T) {
		testRuleUnfoldClosePreservesResolverWhileManagerExecutes(t, true)
	})
}
