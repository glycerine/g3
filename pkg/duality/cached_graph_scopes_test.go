package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestCachedGraphScopesRetractAssumptionsWithoutRemovingGuards(t *testing.T) {
	ctx, _, _, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer factory.Close()
	term, err := ctx.BoolConst("cached-scope-R")
	must(err)
	closeExpr(t, term)
	falsity, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, falsity)
	graph, err := NewRPFP(ctx)
	must(err)
	parent, err := graph.CreateNode(term)
	must(err)
	must(parent.Bound.SetEmpty())
	formula, err := NewTransformer(ctx, nil, nil, falsity)
	must(err)
	defer formula.Close()
	edge, err := graph.CreateEdge(parent, formula, nil)
	must(err)
	pool, err := NewEdgeSolverPool(ctx, factory)
	must(err)
	defer pool.Close()
	state, err := pool.SolverForEdge(edge, true, false, nil)
	must(err)
	scope, err := state.Scope()
	must(err)
	defer scope.Close()
	defer graph.Close()
	cached, err := NewCachedGraphAssertions(graph, scope)
	must(err)
	defer cached.Close()
	profile := NewProfiler(nil)
	ctx.activeProfiler = profile
	defer func() { ctx.activeProfiler = nil }()
	must(profile.TimerStart("outer"))
	outer := profile.current
	defer func() { must(profile.TimerStop("outer")) }()
	check := func(want lbool.LBool) {
		t.Helper()
		status, core, err := cached.Check(nil, true)
		must(err)
		for _, term := range core {
			must(term.Close())
		}
		if status != want {
			t.Fatal("cached status", status, want)
		}
	}
	must(cached.Push())
	must(cached.AssertNode(parent))
	if outer.children["solver add"] != nil || profile.current != outer {
		t.Fatal("node assertion recorded edge solver timer")
	}
	check(lbool.True)
	must(cached.Push())
	// Populate the edge cache first so the failed start below is specifically
	// the solver-add timer, rather than a formula-construction timer.
	formulaValue, err := graph.GetEdgeFormula(edge, 0, false, false)
	must(err)
	must(formulaValue.Close())
	now := profile.now
	profile.now = nil
	if err := cached.AssertEdge(edge, 0, false, false); err == nil {
		t.Fatal("invalid profiler accepted")
	}
	profile.now = now
	if profile.current != outer {
		t.Fatal("failed timer start changed current scope")
	}
	check(lbool.True)
	must(cached.AssertEdge(edge, 0, false, false))
	if outer.children["solver add"] == nil || profile.current != outer {
		t.Fatal("cached assertion missing solver add scope")
	}
	check(lbool.False)
	guards, err := scope.Solver.NumAssertions()
	must(err)
	level, err := scope.Solver.ScopeLevel()
	must(err)
	if guards != 2 || level != 0 {
		t.Fatal("cached scopes changed native scopes", guards, level)
	}
	must(cached.Pop(1))
	if edge.dual != (ast.ASTRef{}) || parent.dual == (ast.ASTRef{}) {
		t.Fatal("cached pop graph records")
	}
	check(lbool.True)
	must(cached.Pop(1))
	if parent.dual != (ast.ASTRef{}) {
		t.Fatal("node dual survived pop")
	}
	check(lbool.True)
	after, err := scope.Solver.NumAssertions()
	must(err)
	if after != guards {
		t.Fatal("cached pop removed guards")
	}
	must(cached.Push())
	must(cached.AssertNode(parent))
	must(cached.AssertEdge(edge, 1, false, false))
	check(lbool.False)
	after, err = scope.Solver.NumAssertions()
	must(err)
	if after != guards {
		t.Fatal("repeated graph assertions created new guards")
	}
	must(cached.Pop(1))
	if edge.dual == (ast.ASTRef{}) {
		t.Fatal("persistent cached formula lost")
	}
	check(lbool.True)
	if err = cached.Pop(1); err == nil {
		t.Fatal("base cached graph scope popped")
	}
	// A closed literal stack reaches the timed add and refuses it. The
	// profiling scope must still be restored on the actual assertion error.
	must(cached.Close())
	must(profile.TimerStart("failed cached add"))
	failureScope := profile.current
	if err = cached.AssertEdge(edge, 0, false, false); err == nil {
		t.Fatal("closed cached assertion accepted")
	}
	if profile.current != failureScope || failureScope.children["solver add"] == nil {
		t.Fatal("failed cached assertion did not restore solver timer")
	}
	must(profile.TimerStop("failed cached add"))
}
