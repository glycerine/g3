package base_test

// Source: src/muz/base/dl_rule.cpp hoist_compound_predicates. The fresh
// variable must survive until its equality is built and be released on failure.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleCompoundHoistingOwnsVariableBeforeReferenceAllocation(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	m := f.manager
	one, err := m.ArithmeticIntegerNumeralText("1")
	ruleMust(t, err)
	sum, err := m.MkArithmeticAdd(f.x, one)
	ruleMust(t, err)
	formula, err := m.MkApp(f.pred, []ast.ASTHandle{sum})
	ruleMust(t, err)
	owner, err := m.NewRef(formula)
	ruleMust(t, err)
	defer func() { ruleMust(t, m.DelRef(&owner)) }()
	// Fill the existing public-reference page so the next owning slot must
	// allocate. The old hoist creates its variable and equality before that slot.
	var pressure []ast.ASTRef
	defer func() {
		for i := range pressure {
			ruleMust(t, m.DelRef(&pressure[i]))
		}
	}()
	for {
		m.Budget().FailAfter(0)
		pin, cause := m.NewRef(ast.ASTHandle{})
		m.Budget().FailAfter(-1)
		if cause != nil {
			if !errors.Is(cause, arena.ErrInjectedFailure) || pin != (ast.ASTRef{}) {
				t.Fatal("reference pressure", pin, cause)
			}
			break
		}
		pressure = append(pressure, pin)
		if len(pressure) > 4096 {
			t.Fatal("reference pressure never reached a physical allocation")
		}
	}
	before := m.Stats()
	m.Budget().FailAfter(0)
	result, cause := f.rules.NewRuleFromFormula(formula, "hoist-failure")
	m.Budget().FailAfter(-1)
	if result != nil || !errors.Is(cause, arena.ErrInjectedFailure) {
		t.Fatal("expected cold reference allocation failure", result, cause)
	}
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatal("failed hoisting lost its fresh variable", before, after)
	}
	result, err = f.rules.NewRuleFromFormula(formula, "hoist-retry")
	ruleMust(t, err)
	defer func() { ruleMust(t, result.Close()) }()
	head, err := result.Head()
	ruleMust(t, err)
	variable, err := m.GetArg(head, 0)
	ruleMust(t, err)
	index, err := m.GetVarIndex(variable)
	ruleMust(t, err)
	count, err := result.TailSize()
	ruleMust(t, err)
	if index != 1 || count != 1 {
		t.Fatal("retry changed hoisted variable or constraints", index, count)
	}
	equality, negative, err := result.Tail(0)
	ruleMust(t, err)
	left, err := m.GetArg(equality, 0)
	ruleMust(t, err)
	right, err := m.GetArg(equality, 1)
	ruleMust(t, err)
	if negative || !((left == variable && right == sum) || (left == sum && right == variable)) {
		t.Fatal("retry lost the original compound argument", left, right, negative)
	}
	ruleMust(t, result.Close())
	if got := m.Stats().LiveReferences; got != before.LiveReferences {
		t.Fatal("retry leaked references", got, before.LiveReferences)
	}
}
