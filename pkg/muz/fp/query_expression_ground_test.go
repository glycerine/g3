package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestFixedpointQueryExpressionGroundConjunction(t *testing.T) {
	f := newQueryExpressionFixture(t)
	pDecl, p := f.predicate("query-ground-P")
	_, q := f.predicate("query-ground-Q")
	queryMust(t, f.engine.AddFact(pDecl, nil))
	query := f.pin(f.manager.MkAndArgs([]ast.ASTHandle{p, q}))
	result, err := f.engine.Query(query)
	queryMust(t, err)
	if result != lbool.False {
		t.Fatal("ground conjunction", result)
	}
}
