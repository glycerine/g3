package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestCachedGreedyReductionDuplicatesTheoryFallbackAndFailure(t *testing.T) {
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
	p, err := ctx.BoolConst("reduce-p")
	must(err)
	closeExpr(t, p)
	negative, err := p.Not()
	must(err)
	closeExpr(t, negative)
	falsity, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, falsity)
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
	state, err := pool.SolverForEdge(edge, false, false, nil)
	must(err)
	scope, err := state.Scope()
	must(err)
	defer scope.Close()
	positive, err := scope.Assumptions.GetAssumptionLits(p, nil)
	must(err)
	defer positive[0].Close()
	neg, err := scope.Assumptions.GetAssumptionLits(negative, nil)
	must(err)
	defer neg[0].Close()
	clone := func(term *Expr) *Expr { value, err := term.Clone(); must(err); return value }
	closeVector := func(terms []*Expr) {
		for _, term := range terms {
			must(term.Close())
		}
	}
	assumptions := []*Expr{clone(neg[0])}
	core := []*Expr{clone(positive[0]), clone(positive[0])}
	must(scope.GreedyReduceCache(&assumptions, &core, nil))
	if len(assumptions) != 1 || len(core) != 1 {
		t.Fatal("duplicate core not reduced", len(core))
	}
	literals := append(append([]*Expr(nil), assumptions...), core...)
	status, verified, err := scope.CheckCore(literals)
	must(err)
	closeVector(verified)
	if status != lbool.False {
		t.Fatal("reduced core no longer UNSAT")
	}
	status, verified, err = scope.CheckCore(assumptions)
	must(err)
	closeVector(verified)
	if status != lbool.True {
		t.Fatal("remaining core literal is unnecessary")
	}
	closeVector(assumptions)
	closeVector(core)
	// A theory guard can establish inconsistency when the initial core cannot.
	assumptions = nil
	core = []*Expr{clone(positive[0])}
	must(scope.GreedyReduceCache(&assumptions, &core, []*Expr{falsity}))
	if len(assumptions) != 1 || len(core) != 0 {
		t.Fatal("theory fallback did not replace unnecessary core")
	}
	closeVector(assumptions)
	assumptions = nil
	core = []*Expr{clone(positive[0])}
	if err = scope.GreedyReduceCache(&assumptions, &core, nil); !errors.Is(err, ErrGreedyReduceFailed) {
		t.Fatal("non-UNSAT reduction did not fail", err)
	}
	if len(core) != 1 || len(assumptions) != 0 {
		t.Fatal("failed verification mutated caller vectors")
	}
	closeVector(core)
}
