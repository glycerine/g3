package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIGoalAssertResetAndSortChecks(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(true, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	boolean := f.pin(f.c.BoolSort())
	x := f.pin(f.c.Constant("goal-x", boolean))
	astTestMust(t, goal.Assert(x))
	if size, e := goal.Size(); e != nil || size != 1 {
		t.Fatal("asserted size", size, e)
	}
	integer := f.arithmeticVariable("goal-i", true)
	if err = goal.Assert(integer); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-Boolean assertion", err)
	}
	astTestMust(t, goal.Reset())
	if size, e := goal.Size(); e != nil || size != 0 {
		t.Fatal("reset size", size, e)
	}
}
