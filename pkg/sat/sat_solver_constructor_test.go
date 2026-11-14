package sat

// Source: src/sat/sat_solver.cpp:34–74,2763–2771; src/sat/sat_solver.h:74–483.

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	"github.com/glycerine/g3/pkg/util/control"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/random"
)

func satSolverCheckFixture(t *testing.T) (*Solver, *uparams.ParamsContext, uparams.ParamsRef) {
	t.Helper()
	ctx, _, _ := paramsTestOwner(t)
	p, e := ctx.NewValue()
	satOK(t, e)
	t.Cleanup(func() { satOK(t, ctx.Del(&p)) })
	runtime, e := NewRuntime(RuntimeConfig{Budget: ctx.BudgetField, ParamsContext: ctx})
	satOK(t, e)
	t.Cleanup(func() { satOK(t, runtime.Close()) })
	limit, e := control.NewReslimit(ctx.BudgetField)
	satOK(t, e)
	t.Cleanup(limit.Close)
	s, e := NewSATSolver(SolverConfig{ParamsContext: ctx, Params: p, RLimit: limit, Budget: ctx.BudgetField, Runtime: runtime})
	satOK(t, e)
	t.Cleanup(func() { satOK(t, s.Close()) })
	return s, ctx, p
}
func TestSATSolverCompleteMemberConstructorHasRealDefaultChildren(t *testing.T) {
	s, ctx, params := satSolverCheckFixture(t)
	if s.Variables.queue == nil || s.Variables.watches == nil || s.Variables.activity == nil || s.Variables.trail == nil || s.simplifier == nil || s.scc == nil || s.asymm == nil || s.probing == nil || !s.probing.constructed || !s.converter.constructed || s.antecedents == nil || s.antecedents.table == nil {
		t.Fatal("full constructor lacks actual source members")
	}
	if s.runtime.liveSolvers != 1 || s.nextSimplify != 0 || s.conflict.conflicts != 0 || s.conflict.conflictsSinceGC != 0 || s.propagation.numCheckpoints != 0 {
		t.Fatal("source ctor body")
	}
	if s.mus.isActive || s.mus.maxNumRestarts != ^uint32(0) {
		t.Fatal("source MUS constructor fields")
	}
	key := "random_seed"
	satOK(t, ctx.SetUIntName(&params, &key, 17))
	satOK(t, s.UpdateParams(&params))
	var expected z3rand.Generator
	expected.SetSeed(17)
	if got, want := s.decision.rng.Next(), expected.Next(); got != want {
		t.Fatal("sole source random member reseeded last", got, want)
	}
	satOK(t, s.Close())
	if s.runtime.liveSolvers != 0 {
		t.Fatal("retained runtime owner after actual completed member cleanup")
	}
}
func TestSATSolverConstructorRefusesForeignRuntimeBeforeActualHeaderReserve(t *testing.T) {
	s, ctx, params := satSolverCheckFixture(t)
	wrong := arena.NewBudget(^uint64(0))
	wrong.FailAfter(0)
	if child, e := NewSATSolver(SolverConfig{ParamsContext: ctx, Params: params, RLimit: s.Limit, Budget: wrong, Runtime: s.runtime}); child != nil || e != ErrSATOwner {
		t.Fatal(child, e)
	}
	if wrong.Used() != 0 || s.runtime.liveSolvers != 1 {
		t.Fatal("constructor domain refusal publication")
	}
}

// Explicit completed-member/uncompleted-wrapper protocol state: a C++ failed
// solver constructor has no solver destructor body. All children here are real
// constructed owners, and the empty clause lists need no ownership repair.
func TestSATSolverUncompletedWrapperUnwindDoesNotRunDestructorBody(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	s.constructed = false
	before := s.Lifecycle.nonLearnedGeneration
	satOK(t, s.Close())
	if s.Lifecycle.nonLearnedGeneration != before {
		t.Fatal("failed constructor ran source destructor body", before, s.Lifecycle.nonLearnedGeneration)
	}
}
