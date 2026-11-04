package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func TestEdgeSolverPoolFirstCreationFlagsAxiomsAndCacheIsolation(t *testing.T) {
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
	term, err := ctx.BoolConst("edge-pool-R")
	must(err)
	closeExpr(t, term)
	p, err := ctx.BoolConst("edge-pool-p")
	must(err)
	closeExpr(t, p)
	negative, err := p.Not()
	must(err)
	closeExpr(t, negative)
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	a, err := graph.CreateNode(term)
	must(err)
	ae, err := graph.CreateLowerBoundEdge(a)
	must(err)
	b, err := graph.CreateNode(term)
	must(err)
	be, err := graph.CreateLowerBoundEdge(b)
	must(err)
	pool, err := NewEdgeSolverPool(ctx, factory)
	must(err)
	defer pool.Close()
	proofMode, err := m.ProofsEnabled()
	must(err)
	first, err := pool.SolverForEdge(ae, false, true, []*Expr{p})
	must(err)
	afterMode, err := m.ProofsEnabled()
	must(err)
	if afterMode != proofMode {
		t.Fatal("solver construction changed manager proof mode")
	}
	same, err := pool.SolverForEdge(ae, true, true, []*Expr{negative})
	must(err)
	if same != first {
		t.Fatal("same edge created another solver")
	}
	firstScope, err := first.Scope()
	must(err)
	count, err := firstScope.Solver.NumAssertions()
	must(err)
	if count != 1 {
		t.Fatal("cached creation reasserted axioms", count)
	}
	status, err := firstScope.Solver.CheckSat(nil)
	must(err)
	if status != lbool.True {
		t.Fatal("first solver axioms", status)
	}
	md, err := firstScope.Solver.AcquireModel()
	must(err)
	if md != nil {
		must(md.DecRef())
		t.Fatal("model flag changed on cached lookup")
	}
	one, err := firstScope.Assumptions.GetAssumptionLits(p, nil)
	must(err)
	must(firstScope.Close())
	restored, err := m.ProofsEnabled()
	must(err)
	if restored != proofMode {
		t.Fatal("solver operation did not restore caller proof mode")
	}
	second, err := pool.SolverForEdge(be, true, false, []*Expr{negative})
	must(err)
	secondScope, err := second.Scope()
	must(err)
	if secondScope.Solver == firstScope.Solver || secondScope.Assumptions == firstScope.Assumptions {
		t.Fatal("different edges shared solver state")
	}
	count, err = secondScope.Solver.NumAssertions()
	must(err)
	if count != 0 {
		t.Fatal("disabled axioms asserted")
	}
	status, err = secondScope.Solver.CheckSat(nil)
	must(err)
	if status != lbool.True {
		t.Fatal("second solver status")
	}
	md, err = secondScope.Solver.AcquireModel()
	must(err)
	if md == nil {
		t.Fatal("model flag not applied at creation")
	}
	must(md.DecRef())
	two, err := secondScope.Assumptions.GetAssumptionLits(p, nil)
	must(err)
	x, err := one[0].Raw()
	must(err)
	y, err := two[0].Raw()
	must(err)
	if x == y {
		t.Fatal("different caches reused fresh literal")
	}
	must(one[0].Close())
	must(two[0].Close())
	must(secondScope.Close())
	must(pool.Close())
	must(pool.Close())
	if _, err = pool.SolverForEdge(ae, false, false, nil); err == nil {
		t.Fatal("closed pool accepted lookup")
	}
	failure := errors.New("injected edge solver construction failure")
	calls := 0
	retryFactory := solver.FactoryFunc(func(manager *ast.ASTManager, params *uparams.ParamsRef, proofs, models, cores bool, logic string) (solver.Solver, error) {
		calls++
		if calls == 1 {
			return nil, failure
		}
		return factory.New(manager, params, proofs, models, cores, logic)
	})
	retryPool, err := NewEdgeSolverPool(ctx, retryFactory)
	must(err)
	defer retryPool.Close()
	retryEdge, err := graph.CreateLowerBoundEdge(a)
	must(err)
	if _, err = retryPool.SolverForEdge(retryEdge, false, false, nil); !errors.Is(err, failure) {
		t.Fatal("solver construction failure was not returned", err)
	}
	retried, err := retryPool.SolverForEdge(retryEdge, true, false, nil)
	must(err)
	if retried == nil || calls != 2 {
		t.Fatal("failed edge solver construction was not retryable", calls)
	}
}
