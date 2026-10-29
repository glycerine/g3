package api

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPITacticApplyResultOwnsIndependentSubgoals(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(true, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	boolean := f.pin(f.c.BoolSort())
	x := f.pin(f.c.Constant("tactic-result-x", boolean))
	astTestMust(t, goal.Assert(x))
	skip, err := f.c.SkipTactic()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, skip.Close()) })

	result, err := skip.Apply(goal)
	astTestMust(t, err)
	if count, countErr := result.NumSubgoals(); countErr != nil || count != 1 {
		t.Fatal("subgoal count", count, countErr)
	}
	text, err := result.String()
	astTestMust(t, err)
	if !strings.Contains(text, "tactic-result-x") {
		t.Fatal("result display", text)
	}
	if _, err = result.Subgoal(1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("out-of-bounds subgoal", err)
	}
	subgoal, err := result.Subgoal(0)
	astTestMust(t, err)
	astTestMust(t, result.Close())
	if size, sizeErr := subgoal.Size(); sizeErr != nil || size != 1 {
		t.Fatal("independent subgoal", size, sizeErr)
	}
	astTestMust(t, subgoal.Close())
	if _, err = result.NumSubgoals(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed result", err)
	}
	if size, sizeErr := goal.Size(); sizeErr != nil || size != 1 {
		t.Fatal("source goal changed", size, sizeErr)
	}
}

func TestAPITacticApplyRejectsForeignGoal(t *testing.T) {
	left := newSMTAPIFixture(t, "ALL", nil)
	right := newSMTAPIFixture(t, "ALL", nil)
	tactic, err := left.c.SkipTactic()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, tactic.Close()) })
	goal, err := right.c.NewGoal(false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	if _, err = tactic.Apply(goal); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("foreign goal", err)
	}
}
