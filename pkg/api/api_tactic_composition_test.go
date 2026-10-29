package api

import "testing"

func TestAPITacticSequentialAlternativeAndConditionalComposition(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	skip, err := f.c.SkipTactic()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, skip.Close()) })
	fail, err := f.c.FailTactic()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, fail.Close()) })
	truth, err := f.c.ConstantProbe(1)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, truth.Close()) })
	falsehood, err := f.c.ConstantProbe(0)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, falsehood.Close()) })

	andThen, err := skip.AndThen(skip)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, andThen.Close()) })
	orElse, err := fail.OrElse(skip)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, orElse.Close()) })
	conditional, err := truth.Conditional(skip, fail)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, conditional.Close()) })
	whenFalse, err := skip.When(falsehood)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, whenFalse.Close()) })
	timed, err := skip.TryFor(100)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, timed.Close()) })
	repeated, err := skip.Repeat(2)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, repeated.Close()) })

	for name, tactic := range map[string]*APITactic{
		"and-then": andThen, "or-else": orElse, "conditional": conditional,
		"when-false": whenFalse, "try-for": timed, "repeat": repeated,
	} {
		result, applyErr := tactic.Apply(goal)
		if applyErr != nil {
			t.Fatalf("%s: %v", name, applyErr)
		}
		count, countErr := result.NumSubgoals()
		if countErr != nil || count != 1 {
			t.Fatalf("%s subgoals: %d, %v", name, count, countErr)
		}
		astTestMust(t, result.Close())
	}

	failIf, err := truth.FailIf()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, failIf.Close()) })
	if result, applyErr := failIf.Apply(goal); applyErr == nil {
		if result != nil {
			astTestMust(t, result.Close())
		}
		t.Fatal("true fail-if tactic succeeded")
	}
}
