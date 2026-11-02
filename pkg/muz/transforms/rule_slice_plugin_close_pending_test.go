package transforms

// Source: dl_mk_slice.cpp slice_proof_converter ownership.
// Its Go converter owners must remain reachable when native cleanup is refused.

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

func TestRuleSliceClosePreservesOwnedProofConverter(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var plugin *RuleSlicePlugin
	active, callbacks, intact := false, 0, true
	var original *SliceProofConverter
	var predicate, derived ast.ASTHandle
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			for i := 0; i < 2; i++ {
				cause := plugin.Close()
				intact = intact && errors.Is(cause, ast.ErrASTPending) && !plugin.closed && plugin.proofConverter == original && plugin.predicates[predicate] == derived && len(original.originals) > 0 && len(original.formulas) > 0
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
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	name, err := symbols.Intern("pending-slice-P")
	must(err)
	predicate, err = m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(predicate))
	name, err = symbols.Intern("pending-slice-Goal")
	must(err)
	goal, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(goal))
	x, err := m.MkVar(0, integer)
	must(err)
	ph, err := m.MkApp(predicate, []ast.ASTHandle{x})
	must(err)
	gh, err := m.MkApp(goal, nil)
	must(err)
	var expected ast.ASTHandle
	for i, spec := range []struct {
		head  ast.ASTHandle
		tails []ast.ASTHandle
	}{{ph, nil}, {gh, []ast.ASTHandle{ph}}} {
		rule, e := rules.NewRule(spec.head, spec.tails, nil, "")
		must(e)
		must(rules.MkRuleAssertedProof(rule))
		proof, found, e := rule.Proof()
		must(e)
		if !found {
			t.Fatal("source proof missing")
		}
		if i == 0 {
			expected = proof
		}
		must(set.Add(rule))
		must(rule.Close())
	}
	must(set.SetOutputPredicate(goal))
	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("invalid fixture")
	}
	plugin = NewRuleSlicePlugin(m, rules, 1)
	plugin.SetGenerateProofTrace(true)
	defer func() {
		if !plugin.closed {
			must(plugin.Close())
		}
	}()
	transformed, err := plugin.Transform(set)
	must(err)
	if transformed == nil {
		t.Fatal("fixture did not slice")
	}
	defer func() { must(transformed.Close()) }()
	var found bool
	derived, found = plugin.DerivedPredicate(predicate)
	if !found {
		t.Fatal("predicate not sliced")
	}
	original = plugin.proofConverter
	if original == nil || len(original.originals) == 0 || len(original.formulas) == 0 {
		t.Fatal("missing proof owners")
	}
	rr, err := transformed.PredicateRules(derived)
	must(err)
	if len(rr) != 1 {
		t.Fatal("sliced rule count", len(rr))
	}
	slicedProof, found, err := rr[0].Proof()
	must(err)
	if !found {
		t.Fatal("sliced proof missing")
	}
	must(rr[0].Close())

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
		t.Fatal("refused close lost proof converter ownership", body, callbacks, intact)
	}
	if plugin.TakeProofConverter() != nil {
		t.Fatal("closing plugin transferred proof converter")
	}
	if _, ok := plugin.DerivedPredicate(predicate); ok {
		t.Fatal("closing plugin published predicate")
	}
	predicates, slices := plugin.PredicateSlices()
	if len(predicates) != 0 || len(slices) != 0 {
		t.Fatal("closing plugin published predicate maps")
	}

	if next, cause := plugin.Transform(set); !errors.Is(cause, base.ErrRuleTransformerOwner) || next != nil {
		t.Fatal("closing plugin accepted transformation", cause)
	}
	converted, err := original.Convert(slicedProof)
	must(err)
	h, err := m.RefGet(converted)
	must(err)
	if h != expected {
		t.Fatal("refused parent close changed original proof")
	}
	must(m.DelRef(&converted))
	must(plugin.Close())
	if cause := plugin.Close(); !errors.Is(cause, base.ErrRuleTransformerOwner) {
		t.Fatal("closed-owner guard changed", cause)
	}
	if !plugin.closed || plugin.proofConverter != nil || plugin.predicates != nil || !original.closed {
		t.Fatal("retry did not release proof converter")
	}
}
