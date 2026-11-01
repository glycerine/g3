package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestLocalEdgeCheckPreservesCallerScopeAndReusesFormula(t *testing.T) {
	ctx, _, s := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("local-edge-R")
	must(err)
	closeExpr(t, term)
	falsity, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, falsity)
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	must(graph.BindSolver(s))
	parent, err := graph.CreateNode(term)
	must(err)
	must(parent.Bound.SetEmpty())
	formula, err := NewTransformer(ctx, nil, nil, falsity)
	must(err)
	defer formula.Close()
	edge, err := graph.CreateEdge(parent, formula, nil)
	must(err)
	must(s.Push()) // Caller-owned scope is independent of graph-local scopes.
	for n := 0; n < 2; n++ {
		status, err := graph.CheckEdge(edge)
		must(err)
		if status != lbool.False {
			t.Fatal("local infeasible edge", status)
		}
		depth, err := s.ScopeLevel()
		must(err)
		if depth != 1 || parent.dual != (ast.ASTRef{}) || edge.dual == (ast.ASTRef{}) {
			t.Fatal("local check scope/cache lifecycle")
		}
	}
	must(s.Pop(1))
}
