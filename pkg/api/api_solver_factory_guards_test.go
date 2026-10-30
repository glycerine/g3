package api

import (
	"errors"
	"testing"
)

func TestAPISolverFactoryTacticOwnershipGuards(t *testing.T) {
	left := newSMTAPIFixture(t, "QF_UF", nil)
	right := newSMTAPIFixture(t, "QF_UF", nil)
	foreign, err := right.c.SkipTactic()
	astTestMust(t, err)
	defer func() { astTestMust(t, foreign.Close()) }()
	if solver, makeErr := left.c.NewSolverFromTactic(foreign); !errors.Is(makeErr, ErrSMTContextOwner) || solver != nil {
		t.Fatal("foreign tactic accepted", solver, makeErr)
	}
	closed, err := left.c.SkipTactic()
	astTestMust(t, err)
	astTestMust(t, closed.Close())
	if solver, makeErr := left.c.NewSolverFromTactic(closed); !errors.Is(makeErr, ErrSMTContextOwner) || solver != nil {
		t.Fatal("closed tactic accepted", solver, makeErr)
	}
	if solver, makeErr := left.c.NewSolverFromTactic(nil); !errors.Is(makeErr, ErrSMTContextOwner) || solver != nil {
		t.Fatal("nil tactic accepted", solver, makeErr)
	}
}
