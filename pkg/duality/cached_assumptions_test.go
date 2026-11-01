package duality

import (
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestCachedAssumptionScopesAndFilteredCores(t *testing.T) {
	ctx, m, _, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer factory.Close()
	p, err := ctx.BoolConst("stack-p")
	must(err)
	closeExpr(t, p)
	negative, err := p.Not()
	must(err)
	closeExpr(t, negative)
	unrelated, err := ctx.BoolConst("stack-unrelated")
	must(err)
	closeExpr(t, unrelated)
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	node, err := graph.CreateNode(p)
	must(err)
	edge, err := graph.CreateLowerBoundEdge(node)
	must(err)
	pool, err := NewEdgeSolverPool(ctx, factory)
	must(err)
	defer pool.Close()
	state, err := pool.SolverForEdge(edge, true, false, nil)
	must(err)
	scope, err := state.Scope()
	must(err)
	defer scope.Close()
	stack, err := NewCachedAssumptions(scope)
	must(err)
	defer stack.Close()
	pos, err := scope.Assumptions.GetAssumptionLits(p, nil)
	must(err)
	defer pos[0].Close()
	neg, err := scope.Assumptions.GetAssumptionLits(negative, nil)
	must(err)
	defer neg[0].Close()
	other, err := scope.Assumptions.GetAssumptionLits(unrelated, nil)
	must(err)
	defer other[0].Close()
	status, core, err := scope.CheckCore([]*Expr{pos[0], neg[0]})
	must(err)
	if status != lbool.False || len(core) != 2 {
		t.Fatal("direct native core", status, len(core))
	}
	for _, term := range core {
		must(term.Close())
	}
	status, core, err = scope.CheckCore([]*Expr{pos[0]})
	must(err)
	if status != lbool.True || len(core) != 0 {
		t.Fatal("SAT core was not empty")
	}
	must(stack.Push())
	must(stack.Assert(p))
	status, core, err = stack.Check([]*Expr{other[0], neg[0], neg[0]}, true)
	must(err)
	if status != lbool.False || len(core) != 2 {
		t.Fatal("filtered transient core", status, len(core))
	}
	want, err := neg[0].Raw()
	must(err)
	for _, term := range core {
		h, err := term.Raw()
		must(err)
		if h != want {
			t.Fatal("active or unrelated assumption leaked into core")
		}
		must(term.Close())
	}
	if len(stack.literals) != 1 {
		t.Fatal("transient assumptions changed active stack")
	}
	must(stack.Push())
	must(stack.Assert(negative))
	status, core, err = stack.Check(nil, true)
	must(err)
	if status != lbool.False || len(core) != 0 {
		t.Fatal("active-only contradiction returned transient core")
	}
	must(stack.Pop(1))
	status, _, err = stack.Check(nil, false)
	must(err)
	if status != lbool.True {
		t.Fatal("inner pop did not retract assumption")
	}
	must(stack.Pop(1))
	status, _, err = stack.Check([]*Expr{neg[0]}, false)
	must(err)
	if status != lbool.True {
		t.Fatal("outer pop did not retract assumption")
	}
	count, err := scope.Solver.NumAssertions()
	must(err)
	depth, err := scope.Solver.ScopeLevel()
	must(err)
	if count != 3 || depth != 0 {
		t.Fatal("cached scopes changed guard solver", count, depth)
	}
	if err = stack.Pop(1); err == nil {
		t.Fatal("assumption stack underflow accepted")
	}
	before := len(stack.literals)
	if before != 0 {
		t.Fatal("assumption stack not empty")
	}
	_ = m
}
