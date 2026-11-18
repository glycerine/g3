package api

import (
	"testing"
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
}

// AUDIT.md api_01-F8: Z3_goal_translate into the same context copies the
// goal.
func TestGoalTranslateSameContext(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	boolean := f.pin(f.c.BoolSort())
	astTestMust(t, goal.Assert(f.pin(f.c.Constant("p", boolean))))
	copied, err := goal.Translate(f.c)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, copied.Close()) })
	original, err := goal.String()
	astTestMust(t, err)
	text, err := copied.String()
	astTestMust(t, err)
	if text != original {
		t.Fatalf("copy %q, original %q", text, original)
	}
}
