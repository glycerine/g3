package api

import (
	"strings"
	"testing"
)

func TestAPIGoalStringOmitsNativeTrailingNewline(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	boolean := f.pin(f.c.BoolSort())
	x := f.pin(f.c.Constant("display-goal-x", boolean))
	astTestMust(t, goal.Assert(x))
	text, err := goal.String()
	astTestMust(t, err)
	if !strings.Contains(text, "display-goal-x") || strings.HasSuffix(text, "\n") {
		t.Fatal("goal display", text)
	}
}
