package fp

// Source: src/muz/transforms/dl_mk_karr_invariants.cpp get_invariants on
// m_inner_ctx (engine datalog).

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestKarrInnerContextQueriesTheInnerDatalogContext(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, e.Close()) })
	inner := e.newKarrInnerContext()
	t.Cleanup(func() { fragmentMust(t, inner.Close()) })
	// An empty rule set saturates; there are no heads to read.
	empty, err := base.NewRuleSet(e.rules)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, empty.Close()) })
	fragmentMust(t, inner.Query(empty, nil))
	if inner.inner == nil {
		t.Fatal("no inner datalog context")
	}
	// An Int column fails check_rules (check_infinite_sorts) in rel_query.
	zero, err := m.ArithmeticIntegerNumeralText("0")
	fragmentMust(t, err)
	integer, err := m.GetSort(zero)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("karr-inner-p")
	fragmentMust(t, err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, e.RegisterPredicate(p))
	head, err := m.MkApp(p, []ast.ASTHandle{zero})
	fragmentMust(t, err)
	rules, err := base.NewRuleSet(e.rules)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, rules.Close()) })
	fact, err := e.rules.NewRule(head, nil, nil, "fact")
	fragmentMust(t, err)
	fragmentMust(t, rules.Add(fact))
	fragmentMust(t, fact.Close())
	err = inner.Query(rules, []ast.ASTHandle{p})
	var property *base.RulePropertyError
	if !errors.As(err, &property) || property.Kind != base.RulePropertyInfiniteSort {
		t.Fatal("inner datalog context accepted an Int column", err)
	}
	formula, err := inner.Formula(p)
	fragmentMust(t, err)
	if formula != (ast.ASTRef{}) {
		t.Fatal("rejected query left a relation")
	}
}
