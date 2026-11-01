package fp

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSpacerRelationQueryReversesMixedSortHead(t *testing.T) {
	m, _ := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, e.Close()) }()
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	zero, err := m.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	fragmentMust(t, err)
	integer, err := m.GetSort(zero)
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("spacer-mixed-query")
	fragmentMust(t, err)
	relation, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, e.RegisterPredicate(relation))
	query, rule, err := e.makeSpacerRelationQuery(relation)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, rule.Close()) }()
	for i, want := range []ast.ASTHandle{integer, boolean} {
		got, err := m.GetDomain(query, uint32(i))
		fragmentMust(t, err)
		if got != want {
			t.Fatalf("query head domain %d has incorrect sort", i)
		}
	}
	head, err := rule.Head()
	fragmentMust(t, err)
	tail, negated, err := rule.Tail(0)
	fragmentMust(t, err)
	if negated {
		t.Fatal("relation query tail is negated")
	}
	for i := uint32(0); i < 2; i++ {
		arg, err := m.GetArg(head, i)
		fragmentMust(t, err)
		index, err := m.GetVarIndex(arg)
		fragmentMust(t, err)
		if index != 1-i {
			t.Fatalf("head variable %d has index %d", i, index)
		}
		arg, err = m.GetArg(tail, i)
		fragmentMust(t, err)
		index, err = m.GetVarIndex(arg)
		fragmentMust(t, err)
		if index != i {
			t.Fatalf("tail variable %d has index %d", i, index)
		}
	}
}
