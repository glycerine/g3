package base_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExistentialRuleHeadReservesEqualityOwnerBeforeConstruction(t *testing.T) {
	f := newRulePropertiesFixture(t)
	m := f.manager
	boolean, err := m.MkBoolSort()
	ruleMust(t, err)
	name, err := m.Symbols.Intern("head-var")
	ruleMust(t, err)
	head, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{name}, Body: f.q})
	ruleMust(t, err)
	root, err := m.NewRef(head)
	ruleMust(t, err)
	defer func() { ruleMust(t, m.DelRef(&root)) }()
	_, err = m.MkTrue()
	ruleMust(t, err)
	var pressure []ast.ASTRef
	defer func() {
		for i := range pressure {
			ruleMust(t, m.DelRef(&pressure[i]))
		}
	}()
	for {
		m.Budget().FailAfter(0)
		ref, cause := m.NewRef(ast.ASTHandle{})
		m.Budget().FailAfter(-1)
		if cause != nil {
			if !errors.Is(cause, arena.ErrInjectedFailure) || ref != (ast.ASTRef{}) {
				t.Fatal("reference pressure", ref, cause)
			}
			break
		}
		pressure = append(pressure, ref)
		if len(pressure) > 4096 {
			t.Fatal("reference pressure did not exhaust the page")
		}
	}
	before := m.Stats()
	m.Budget().FailAfter(0)
	rule, cause := f.rules.NewRuleFromFormula(head, "")
	m.Budget().FailAfter(-1)
	if rule != nil || !errors.Is(cause, arena.ErrInjectedFailure) {
		t.Fatal("expected reference reservation failure", rule, cause)
	}
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatal("failed conversion lost an owner", before, after)
	}
	rule, cause = f.rules.NewRuleFromFormula(head, "")
	var invalid *base.InvalidRuleHeadError
	if rule != nil || !errors.As(cause, &invalid) {
		t.Fatal("retry did not report invalid head", rule, cause)
	}
	if got := m.Stats().LiveReferences; got != before.LiveReferences {
		t.Fatal("retry leaked references", got, before.LiveReferences)
	}
}
