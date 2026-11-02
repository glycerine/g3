package base

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRuleClosePreservesOwnersDuringASTInvocation(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var rule, shared *Rule
	active, callbacks, intact := false, 0, true
	var closeErr error
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			if shared != nil {
				must(shared.Close())
				shared = nil
			}
			state := rule.state
			if state == nil {
				intact = false
				return f
			}
			head, proof, tails, refs, accounting := state.headReference, state.proofReference, len(state.tail), state.references, state.accounting
			for i := 0; i < 2; i++ {
				closeErr = rule.Close()
				intact = intact && errors.Is(closeErr, ast.ErrASTPending) && !rule.closed && rule.state == state && state.headReference == head && state.proofReference == proof && len(state.tail) == tails && state.references == refs && state.accounting == accounting
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
	rules, err := NewRuleManager(m)
	must(err)
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if shared != nil {
			must(shared.Close())
		}
		if rule != nil {
			must(rule.Close())
		}
		must(rules.Close())
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("pending rule fixture leaked bytes", budget.Used())
		}
	}()
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	head, err := m.MkFreshConst("pending-rule-head", boolean)
	must(err)
	declaration, err := m.GetDecl(head)
	must(err)
	must(rules.RegisterPredicate(declaration))
	auxiliary, err := m.MkFreshConst("pending-rule-tail", boolean)
	must(err)
	rule, err = rules.NewRule(head, []ast.ASTHandle{auxiliary}, nil, "")
	must(err)
	must(rules.MkRuleAssertedProof(rule))
	shared, err = rule.Retain()
	must(err)
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
		t.Fatal("pending close lost rule owners", body, callbacks, intact, closeErr)
	}
	actual, err := rule.Head()
	must(err)
	if actual != head {
		t.Fatal("pending close changed head")
	}
	proof, found, err := rule.Proof()
	must(err)
	if !found {
		t.Fatal("pending close lost proof")
	}
	_, err = m.GetProofFact(proof)
	must(err)
	tail, _, err := rule.Tail(0)
	must(err)
	if tail != auxiliary {
		t.Fatal("pending close changed tail")
	}
	must(rule.Close())
	must(rule.Close())
	// Exercise a real post-admission refusal through active cost recording on
	// the sole parent rule owned by the child's accounting object.
	head, err = m.MkApp(declaration, nil)
	must(err)
	parent, err := rules.NewRule(head, nil, nil, "accounting-parent")
	must(err)
	defer func() { must(parent.Close()) }()
	child, err := rules.NewRule(head, nil, nil, "accounting-child")
	must(err)
	defer func() { must(child.Close()) }()
	must(rules.MkRuleAssertedProof(child))
	must(child.SetAccountingParentRule(parent))
	recorder := NewCostRecorder()
	defer func() { must(recorder.Close()) }()
	must(recorder.Start(parent.state.accounting))
	must(parent.Close())
	childState := child.state
	ownedParent := childState.accounting.parentRule
	childHead, childProof := childState.headReference, childState.proofReference
	refusal := child.Close()
	if !errors.Is(refusal, ErrCosts) || child.closed || child.state != childState || childState.references != 1 || childState.headReference != childHead || childState.proofReference != childProof || childState.accounting == nil || childState.accounting.closed || childState.accounting.parentRule != ownedParent || ownedParent.closed {
		t.Fatal("accounting refusal lost rule owners", refusal)
	}
	must(recorder.Finish())
	must(child.Close())
	must(child.Close())
}
