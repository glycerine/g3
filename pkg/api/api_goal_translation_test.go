package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIGoalTranslationOwnsTargetFormulas(t *testing.T) {
	source := newSMTAPIFixture(t, "ALL", nil)
	target, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, target.Close()) })
	astTestMust(t, target.SetLogic("ALL"))
	goal, err := source.c.NewGoal(true, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	boolean := source.pin(source.c.BoolSort())
	x := source.pin(source.c.Constant("translated-goal-x", boolean))
	astTestMust(t, goal.Assert(x))

	translated, err := goal.Translate(target)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, translated.Close()) })
	targetManager, err := target.Manager()
	astTestMust(t, err)
	ref, err := translated.Formula(0)
	astTestMust(t, err)
	defer func() { astTestMust(t, targetManager.DelRef(&ref)) }()
	handle, err := targetManager.RefGet(ref)
	astTestMust(t, err)
	if source.m.IsLiveHandle(handle) {
		t.Fatal("translated formula provenance")
	}
	if _, err = goal.Translate(source.c); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("same-context translation", err)
	}
}
