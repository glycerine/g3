package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestFixedpointQueryExpressionFreeVariableMatchesAnyTuple(t *testing.T) {
	f := newQueryExpressionFixture(t)
	boolean := f.pin(f.manager.MkBoolSort())
	relation, _ := f.predicate("query-free-R", boolean)
	truth := f.pin(f.manager.MkTrue())
	queryMust(t, f.engine.AddFact(relation, []ast.ASTHandle{truth}))
	variable := f.pin(f.manager.MkVar(0, boolean))
	query := f.pin(f.manager.MkApp(relation, []ast.ASTHandle{variable}))
	result, err := f.engine.Query(query)
	queryMust(t, err)
	if result != lbool.True {
		t.Fatal("free-variable query", result)
	}
}

func TestFixedpointQueryExpressionCompactsVariableGaps(t *testing.T) {
	f := newQueryExpressionFixture(t)
	boolean := f.pin(f.manager.MkBoolSort())
	relation, _ := f.predicate("query-gaps-R", boolean, boolean)
	truth := f.pin(f.manager.MkTrue())
	falsity := f.pin(f.manager.MkFalse())
	queryMust(t, f.engine.AddFact(relation, []ast.ASTHandle{truth, falsity}))
	x := f.pin(f.manager.MkVar(2, boolean))
	y := f.pin(f.manager.MkVar(5, boolean))
	query := f.pin(f.manager.MkApp(relation, []ast.ASTHandle{x, y}))
	result, err := f.engine.QueryExpression(query)
	queryMust(t, err)
	if result != lbool.True {
		t.Fatal("gapped query", result)
	}
	repeated := f.pin(f.manager.MkApp(relation, []ast.ASTHandle{x, x}))
	result, err = f.engine.QueryExpression(repeated)
	queryMust(t, err)
	if result != lbool.False {
		t.Fatal("gapped repeated-variable query", result)
	}
}
