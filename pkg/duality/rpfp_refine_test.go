package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDualityCollapseTreePreservesSourceConstraintOrder(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var terms []*Expr
	for _, name := range []string{"collapse-root", "collapse-extra", "collapse-left", "collapse-left-extra", "collapse-grandchild", "collapse-right"} {
		term, err := ctx.BoolConst(name)
		must(err)
		closeExpr(t, term)
		terms = append(terms, term)
	}
	before := m.Stats()
	grandchild, err := NewTermTree(terms[4], nil)
	must(err)
	left, err := NewTermTree(terms[2], []*TermTree{grandchild})
	must(err)
	must(left.AddTerm(terms[3]))
	right, err := NewTermTree(terms[5], nil)
	must(err)
	root, err := NewTermTree(terms[0], []*TermTree{left, right})
	must(err)
	must(root.AddTerm(terms[1]))
	must(CollapseTermTree(root))
	if len(root.children) != 0 || len(root.terms) != 5 || !left.closed || !right.closed || !grandchild.closed {
		t.Fatal("collapse lifecycle")
	}
	for n, term := range terms[1:] {
		got, err := root.terms[n].Raw()
		must(err)
		want, err := term.Raw()
		must(err)
		if got != want {
			t.Fatal("collapse order", n)
		}
	}
	must(root.Close())
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatal("collapse leaked", before, after)
	}
}
func TestDualitySingleNodeRefinementUsesSurroundingContext(t *testing.T) {
	ctx, m, s, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer manager.Close()
	parentTerm, err := ctx.BoolConst("refine-parent")
	must(err)
	closeExpr(t, parentTerm)
	childTerm, err := ctx.BoolConst("refine-child")
	must(err)
	closeExpr(t, childTerm)
	childDecl, err := childTerm.Decl()
	must(err)
	closeDecl(t, childDecl)
	falsity, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, falsity)
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(s))
	parent, err := g.CreateNode(parentTerm)
	must(err)
	must(parent.Bound.SetEmpty())
	child, err := g.CreateNode(childTerm)
	must(err)
	parentF, err := NewTransformer(ctx, []*FuncDecl{childDecl}, nil, childTerm)
	must(err)
	defer parentF.Close()
	childF, err := NewTransformer(ctx, nil, nil, falsity)
	must(err)
	defer childF.Close()
	parentEdge, err := g.CreateEdge(parent, parentF, []*Node{child})
	must(err)
	childEdge, err := g.CreateEdge(child, childF, nil)
	must(err)
	must(g.Push())
	must(g.AssertNode(parent))
	must(g.AssertEdge(parentEdge, 0, false, false))
	must(g.AssertEdge(childEdge, 0, false, false))
	status, err := g.SolveSingleNode(parent, child, manager, nil)
	must(err)
	if status != lbool.False {
		t.Fatal("single node solve", status)
	}
	full, err := parent.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("refinement changed surrounding parent")
	}
	learned, err := child.Annotation.Formula()
	must(err)
	defer learned.Close()
	must(g.Pop(1))
	must(s.Push())
	h, err := learned.Raw()
	must(err)
	must(s.Assert(h))
	status, err = s.CheckSat(nil)
	must(err)
	if status != lbool.False {
		t.Fatal("child annotation not learned", status)
	}
	must(s.Pop(1))
}
