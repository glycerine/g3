package tactic

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestTacticResultGoalReferenceTransfersDirectOwner(t *testing.T) {
	m := astTestManager(t)
	goal := goalTestNew(t, m, false, false)
	result := &TacticResult{}
	astTestMust(t, result.Prepare(m))
	astTestMust(t, result.TakeGoal(goal))
	shared, err := result.AcquireGoalReference(0)
	astTestMust(t, err)
	astTestMust(t, result.Close())
	if shared != goal || goal.Closed || goal.RefCount != 1 {
		t.Fatal("direct result owner did not transfer into live counted goal")
	}
	astTestMust(t, shared.DecRef())
	if !goal.Closed {
		t.Fatal("last public reference did not destroy transferred goal")
	}
	if _, err = result.AcquireGoalReference(0); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("closed result goal acquisition", err)
	}
}

func TestTacticResultGoalReferencePreservesBorrowerAndBounds(t *testing.T) {
	m := astTestManager(t)
	goal := goalTestNew(t, m, false, false)
	astTestMust(t, goal.IncRef())
	result := &TacticResult{}
	astTestMust(t, result.Prepare(m))
	astTestMust(t, result.AddGoal(goal))
	if _, err := result.AcquireGoalReference(1); !errors.Is(err, arena.ErrBounds) || goal.RefCount != 1 {
		t.Fatal("out-of-range acquisition changed goal reference", err, goal.RefCount)
	}
	shared, err := result.AcquireGoalReference(0)
	astTestMust(t, err)
	astTestMust(t, result.Close())
	astTestMust(t, shared.DecRef())
	if goal.Closed || goal.RefCount != 1 {
		t.Fatal("public result reference released the borrowed original goal")
	}
	astTestMust(t, goal.DecRef())
}
