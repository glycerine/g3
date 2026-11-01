package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExpressionQueryAllocationFailureInvalidatesSnapshot(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e, m := f.engine, f.manager
	fragmentMust(t, e.SelectEngine("spacer"))
	declaration := f.declarations["p"]
	status, err := e.QueryPredicate(declaration)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("initial query", status)
	}
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	query, err := m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: f.applications["p"]})
	fragmentMust(t, err)
	queryRef, err := m.NewRef(query)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, m.DelRef(&queryRef)) }()
	budget := m.Budget()
	defer budget.FailAfter(-1)
	budget.FailAfter(0)
	_, err = e.QueryExpression(query)
	budget.FailAfter(-1)
	if err == nil {
		t.Fatal("injected query allocation failure was ignored")
	}
	if _, err = e.LastResult(); err != ErrPropositionalEngine {
		t.Fatal("failed allocation left an accessible result", err)
	}
	status, err = e.QueryPredicate(declaration)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("query did not recover after allocation failure", status)
	}
}
