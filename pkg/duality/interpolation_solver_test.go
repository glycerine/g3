package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDualityNativeTreeInterpolation(t *testing.T) {
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
	p, err := ctx.BoolConst("interpolate-p")
	must(err)
	closeExpr(t, p)
	negative, err := p.Not()
	must(err)
	closeExpr(t, negative)
	truth, err := ctx.BoolVal(true)
	must(err)
	closeExpr(t, truth)
	left, err := NewTermTree(p, nil)
	must(err)
	right, err := NewTermTree(negative, nil)
	must(err)
	tree, err := NewTermTree(truth, []*TermTree{left, right})
	must(err)
	defer tree.Close()
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(s))
	status, learned, err := g.InterpolateTree(manager, tree, nil, false, nil)
	must(err)
	if status != lbool.False || learned == nil || len(learned.children) != 2 || g.dualModel != nil {
		t.Fatal("tree interpolation result", status)
	}
	defer learned.Close()
	for n, source := range []*Expr{p, negative} {
		must(s.Push())
		a, err := source.Raw()
		must(err)
		must(s.Assert(a))
		neg, err := learned.children[n].term.Not()
		must(err)
		h, err := neg.Raw()
		must(err)
		must(s.Assert(h))
		must(neg.Close())
		got, err := s.CheckSat(nil)
		must(err)
		if got != lbool.False {
			t.Fatal("leaf does not imply its interpolant", n, got)
		}
		must(s.Pop(1))
	}
	must(s.Push())
	for _, child := range learned.children {
		h, err := child.term.Raw()
		must(err)
		must(s.Assert(h))
	}
	got, err := s.CheckSat(nil)
	must(err)
	if got != lbool.False {
		t.Fatal("interpolants are consistent")
	}
	must(s.Pop(1))
	must(s.Push())
	for _, term := range []*Expr{p, negative} {
		h, err := term.Raw()
		must(err)
		must(s.Assert(h))
	}
	status, incremental, err := g.InterpolateTree(manager, tree, nil, true, nil)
	must(err)
	if status != lbool.False || incremental == nil {
		t.Fatal("incremental interpolation result")
	}
	must(incremental.Close())
	level, err := s.ScopeLevel()
	must(err)
	if level != 1 {
		t.Fatal("incremental interpolation changed scopes")
	}
	must(s.Pop(1))
	// SAT returns a counterexample model and no interpolant tree.
	satTree, err := NewTermTree(p, nil)
	must(err)
	defer satTree.Close()
	status, answer, err := g.InterpolateTree(manager, satTree, nil, false, nil)
	must(err)
	if status != lbool.True || answer != nil || g.dualModel == nil {
		t.Fatal("SAT interpolation result", status)
	}
	count, err := s.NumAssertions()
	must(err)
	if count != 0 {
		t.Fatal("nonincremental interpolation left assertions")
	}
}

func TestDualityGraphSolveLearnsAnnotation(t *testing.T) {
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
	term, err := ctx.BoolConst("solve-R")
	must(err)
	closeExpr(t, term)
	falsity, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, falsity)
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(s))
	node, err := g.CreateNode(term)
	must(err)
	must(node.Bound.SetEmpty())
	transformer, err := NewTransformer(ctx, nil, nil, falsity)
	must(err)
	defer transformer.Close()
	edge, err := g.CreateEdge(node, transformer, nil)
	must(err)
	must(g.Push())
	must(g.AssertNode(node))
	must(g.AssertEdge(edge, 0, false, false))
	status, err := g.Solve(node, manager, nil)
	must(err)
	if status != lbool.False {
		t.Fatal("expected safe graph", status)
	}
	annotation, err := node.Annotation.Formula()
	must(err)
	defer annotation.Close()
	must(g.Pop(1))
	must(s.Push())
	h, err := annotation.Raw()
	must(err)
	must(s.Assert(h))
	status, err = s.CheckSat(nil)
	must(err)
	if status != lbool.False {
		t.Fatal("decoded graph annotation was not false", status)
	}
	must(s.Pop(1))
	must(g.DeleteEdge(edge))
	must(node.Annotation.SetFull())
	lower, err := g.CreateLowerBoundEdge(node)
	must(err)
	must(g.Push())
	must(g.AssertNode(node))
	must(g.AssertEdge(lower, 0, false, false))
	status, err = g.Solve(node, manager, nil)
	must(err)
	full, err := node.Annotation.IsFull()
	must(err)
	if status != lbool.True || g.dualModel == nil || !full {
		t.Fatal("counterexample graph solve", status)
	}
	must(g.Pop(1))
}
