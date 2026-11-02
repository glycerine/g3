package duality

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestRPFPCheckNativeAssumptionCoresAndModelReplacement(t *testing.T) {
	ctx, _, s := dualityNativeSolverFixture(t)
	profile := NewProfiler(nil)
	ctx.activeProfiler = profile
	defer func() {
		if profile.current != profile.top || profile.top.children["Check"] == nil {
			t.Fatal("native phase timer scope", "Check")
		}
	}()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	parentTerm, err := ctx.BoolConst("check-parent")
	must(err)
	closeExpr(t, parentTerm)
	childTerm, err := ctx.BoolConst("check-child")
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
	irrelevant, err := g.CreateNode(childTerm)
	must(err)
	transformer, err := NewTransformer(ctx, []*FuncDecl{childDecl}, nil, childTerm)
	must(err)
	defer func() { must(transformer.Close()) }()
	edge, err := g.CreateEdge(parent, transformer, []*Node{child})
	must(err)
	must(g.Push())
	must(g.AssertNode(parent))
	must(g.AssertEdge(edge, 0, false, true))
	core := []*Node{irrelevant}
	status, err := g.Check(parent, nil, &core)
	must(err)
	if status != lbool.True || g.dualModel == nil || len(core) != 1 || core[0] != irrelevant {
		t.Fatal("unassumed check model/core behavior")
	}
	old := g.dualModel
	status, err = g.Check(parent, []*Node{irrelevant, child}, &core)
	must(err)
	if status != lbool.False || len(core) != 1 || core[0] != child || g.dualModel != nil {
		t.Fatal("assumption core/model replacement", status, len(core))
	}
	status, err = g.Check(parent, []*Node{irrelevant}, &core)
	must(err)
	if status != lbool.True || len(core) != 0 || g.dualModel == nil {
		t.Fatal("SAT did not clear requested core")
	}
	old = g.dualModel
	flag, err := g.UnderapproxFlag(child)
	must(err)
	defer flag.Close()
	status, err = g.CheckUpdateModel(parent, []*Expr{flag})
	must(err)
	if status != lbool.False || g.dualModel != old {
		t.Fatal("update check lost previous model")
	}
	status, err = g.CheckUpdateModel(parent, nil)
	must(err)
	if status != lbool.True || g.dualModel == nil {
		t.Fatal("update check did not acquire model")
	}
	status, err = g.Check(parent, []*Node{child}, nil)
	must(err)
	if status != lbool.False || g.dualModel != nil {
		t.Fatal("check without requested core")
	}
	must(g.Pop(1))
	must(g.Close())
}
