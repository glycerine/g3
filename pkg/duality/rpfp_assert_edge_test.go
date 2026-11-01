package duality

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestRPFPEdgeAssertionsPersistenceAndUnderapproximation(t *testing.T) {
	for _, persist := range []int{0, 1, 99} {
		t.Run(fmt.Sprint(persist), func(t *testing.T) {
			ctx, _, s := dualityNativeSolverFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			parentTerm, err := ctx.BoolConst("assert-parent")
			must(err)
			closeExpr(t, parentTerm)
			childTerm, err := ctx.BoolConst("assert-child")
			must(err)
			closeExpr(t, childTerm)
			childDecl, err := childTerm.Decl()
			must(err)
			closeDecl(t, childDecl)
			g, err := NewRPFP(ctx)
			must(err)
			defer func() { must(g.Close()) }()
			must(g.BindSolver(s))
			parent, err := g.CreateNode(parentTerm)
			must(err)
			must(parent.Bound.SetEmpty())
			child, err := g.CreateNode(childTerm)
			must(err)
			transformer, err := NewTransformer(ctx, []*FuncDecl{childDecl}, nil, childTerm)
			must(err)
			defer func() { must(transformer.Close()) }()
			edge, err := g.CreateEdge(parent, transformer, []*Node{child})
			must(err)
			must(g.Push())
			must(g.AssertNode(parent))
			must(g.Push())
			must(g.AssertEdge(edge, persist, false, true))
			flag, err := g.UnderapproxFlag(child)
			must(err)
			defer flag.Close()
			fh, err := flag.Raw()
			must(err)
			check := func(args []ast.ASTHandle, want lbool.LBool) {
				t.Helper()
				got, err := s.CheckSat(args)
				must(err)
				if got != want {
					t.Fatal("native result", got, want)
				}
			}
			check(nil, lbool.True)
			check([]ast.ASTHandle{fh}, lbool.False)
			cached := edge.dual
			// Cached formula construction ignores later flags, matching source.
			again, err := g.GetEdgeFormula(edge, 0, true, false)
			must(err)
			must(again.Close())
			if edge.dual != cached {
				t.Fatal("cached formula replaced")
			}
			must(g.Pop(1))
			check([]ast.ASTHandle{fh}, lbool.True)
			if persist == 0 && edge.dual != (ast.ASTRef{}) {
				t.Fatal("nonpersistent edge cache survived")
			}
			if persist != 0 && edge.dual != cached {
				t.Fatal("persistent edge cache lost")
			}
			must(g.Push())
			must(g.AssertEdge(edge, 0, false, true))
			check([]ast.ASTHandle{fh}, lbool.False)
			must(g.Pop(1))
			must(g.Pop(1))
			check(nil, lbool.True)
			if persist == 1 && edge.dual != (ast.ASTRef{}) {
				t.Fatal("cache outlived persistence scope")
			}
			must(child.Annotation.SetEmpty())
			must(g.Push())
			must(g.AssertNode(parent))
			must(g.AssertEdge(edge, 0, true, false))
			check(nil, lbool.False)
			if len(edge.constraints) != 1 {
				t.Fatal("child annotation was not recorded")
			}
			must(g.Pop(1))
			check(nil, lbool.True)
			// True lower-bound edges take the source early-return path.
			must(child.Annotation.SetFull())
			lower, err := g.CreateLowerBoundEdge(child)
			must(err)
			count, err := s.NumAssertions()
			must(err)
			must(g.AssertEdge(lower, 0, false, true))
			after, err := s.NumAssertions()
			must(err)
			if count != after || lower.dual != (ast.ASTRef{}) {
				t.Fatal("true edge was asserted")
			}

		})
	}
}
