package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func TestRPFPNativeSolverScopesAndConstraintReplay(t *testing.T) {
	ctx, _, s := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("scope-R")
	must(err)
	closeExpr(t, term)
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(s))
	node, err := g.CreateNode(term)
	must(err)
	must(node.Bound.SetEmpty())
	edge, err := g.CreateLowerBoundEdge(node)
	must(err)
	check := func(want lbool.LBool) {
		t.Helper()
		got, err := s.CheckSat(nil)
		must(err)
		if got != want {
			t.Fatal("native solver result", got, want)
		}
	}
	check(lbool.True)
	must(g.Push())
	must(g.AssertNode(node))
	must(g.AssertNode(node))
	count, err := s.NumAssertions()
	must(err)
	if count != 1 {
		t.Fatal("node asserted more than once", count)
	}
	b, vars, err := g.RedVars(node)
	must(err)
	if len(vars) != 0 {
		t.Fatal("unexpected parameters")
	}
	defer b.Close()
	negative, err := b.Not()
	must(err)
	defer negative.Close()
	check(lbool.True)
	must(g.Push())
	must(g.ConstrainEdgeLocalized(edge, negative))
	check(lbool.False)
	must(g.PopPush())
	check(lbool.False)
	if len(edge.constraints) != 1 {
		t.Fatal("replay duplicated graph constraints")
	}
	must(g.Pop(1))
	check(lbool.True)
	if len(edge.constraints) != 0 {
		t.Fatal("pop did not remove edge constraint")
	}
	must(g.Pop(1))
	if node.dual != (ast.ASTRef{}) || len(g.scopes) != 1 {
		t.Fatal("node dual survived pop")
	}
	// Reassertion after pop must reconstruct the cached upper bound.
	must(g.Push())
	must(g.AssertNode(node))
	check(lbool.True)
	must(g.ConstrainEdgeLocalized(edge, negative))
	check(lbool.False)
	must(g.Pop(1))
	check(lbool.True)
	must(node.Annotation.SetEmpty())
	must(g.Push())
	must(g.AssertNode(node))
	must(g.ConstrainParent(edge, node))
	check(lbool.False)
	must(g.Pop(1))
	check(lbool.True)
	falsity, err := ctx.BoolVal(false)
	must(err)
	must(g.Push())
	must(g.ConstrainEdge(edge, falsity))
	must(falsity.Close())
	check(lbool.False)
	must(g.Pop(1))
	check(lbool.True)
	if err = g.Pop(1); err == nil {
		t.Fatal("popped base scope")
	}
	must(g.Close())
}

func dualityNativeSolverFixture(t *testing.T) (*Context, *ast.ASTManager, solver.Solver) {
	t.Helper()
	ctx, config, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(m.RegisterDeclarationPlugins())
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	must(err)
	t.Cleanup(func() { must(catalog.Close()) })
	globals, err := uparams.NewGlobalParamsService(m.Symbols, config.params, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	must(err)
	t.Cleanup(func() { must(globals.Close()) })
	must(globals.InitializeSource())
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: config.params, Globals: globals})
	must(err)
	t.Cleanup(func() { must(factory.Close()) })
	s, err := factory.New(m, nil, false, true, true, "")
	must(err)
	t.Cleanup(func() { must(s.Close()) })

	return ctx, m, s
}
