package spacer

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

func TestReachFactClosePreservesOwnersDuringASTInvocation(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var fact, shared *ReachFact
	var selection RuleSelection
	active, callbacks, intact := false, 0, true
	var closeErr error
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			selected := selection.Rule
			selectionErr := selection.Close()
			intact = intact && errors.Is(selectionErr, ast.ErrASTPending) && selection.Rule == selected && selected != nil && len(selection.ReachPredUsed) == 1 && selection.NumReuseReach == 1 && selection.IsConcrete

			if shared != nil {
				must(shared.Close())
				shared = nil
			}
			state := fact.state
			if state == nil {
				intact = false
				return f
			}
			root, aux, justifications, refs := state.fact, len(state.auxiliaries), len(state.justifications), state.references
			for i := 0; i < 2; i++ {
				closeErr = fact.Close()
				intact = intact && errors.Is(closeErr, ast.ErrASTPending) && !fact.closed && fact.state == state && state.fact == root && len(state.auxiliaries) == aux && len(state.justifications) == justifications && state.references == refs
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
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if shared != nil {
			must(shared.Close())
		}
		if fact != nil {
			must(fact.Close())
		}
		must(selection.Close())
		must(rules.Close())
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("pending reach fact fixture leaked bytes", budget.Used())
		}
	}()
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	head, err := m.MkFreshConst("pending-reach-head", boolean)
	must(err)
	declaration, err := m.GetDecl(head)
	must(err)
	must(rules.RegisterPredicate(declaration))
	rule, err := rules.NewRule(head, nil, nil, "")
	must(err)
	auxiliary, err := m.MkFreshConst("pending-reach-aux", boolean)
	must(err)
	child, err := NewReachFact(m, rule, head, []ast.ASTHandle{auxiliary}, true)
	must(err)
	fact, err = NewReachFact(m, rule, head, []ast.ASTHandle{auxiliary}, false)
	must(err)
	must(fact.AddJustification(child))
	must(child.Close())
	must(rule.Close())
	shared, err = fact.Retain()
	must(err)
	selected, err := rules.NewRule(head, nil, nil, "pending-selection")
	must(err)
	selection = RuleSelection{Rule: selected, IsConcrete: true, ReachPredUsed: []bool{true}, NumReuseReach: 1}

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
	if body == nil || callbacks == 0 || !intact || !errors.Is(closeErr, ast.ErrASTPending) {
		t.Fatal("pending close lost reach fact owners", body, callbacks, intact, closeErr)
	}
	actual, err := fact.Fact()
	must(err)
	if actual != head {
		t.Fatal("pending close changed fact")
	}
	justifications, err := fact.Justifications()
	must(err)
	if len(justifications) != 1 {
		t.Fatal("pending close lost justification")
	}
	actual, err = justifications[0].Fact()
	must(err)
	if actual != head {
		t.Fatal("pending close changed justification")
	}
	must(justifications[0].Close())
	must(fact.Close())
	must(fact.Close())
	must(selection.Close())
	must(selection.Close())
}
