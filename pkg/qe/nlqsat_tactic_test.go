package qe

import "testing"

func TestNLQSATTacticClosedExistentialAndFalseGoals(t *testing.T) {
	t.Run("existential", func(t *testing.T) {
		fixture := qsatPrepareFixture(t, "exists")
		f := fixture.f
		tactic, err := NewNLQSATTactic(f.r.manager, f.r.config)
		fmMust(t, err)
		t.Cleanup(func() { fmMust(t, tactic.Close()) })
		goal := f.goal(false, false)
		result := f.result()
		fmMust(t, goal.Assert(fixture.original))
		fmMust(t, tactic.Apply(goal, result))
		size, err := goal.Size()
		fmMust(t, err)
		if size != 0 || len(result.Goals) != 1 || result.Goals[0] != goal {
			t.Fatal("existential goal was not decided SAT", size, len(result.Goals))
		}
	})
	t.Run("false", func(t *testing.T) {
		f := newFMFixture(t)
		tactic, err := NewNLQSATTactic(f.r.manager, f.r.config)
		fmMust(t, err)
		t.Cleanup(func() { fmMust(t, tactic.Close()) })
		goal := f.goal(false, false)
		result := f.result()
		falsity, err := f.r.manager.MkFalse()
		fmMust(t, err)
		fmMust(t, goal.Assert(falsity))
		fmMust(t, tactic.Apply(goal, result))
		if !goal.IsDecidedUnsat() || len(result.Goals) != 1 {
			t.Fatal("false goal was not decided UNSAT", goal.IsDecidedUnsat(), len(result.Goals))
		}
	})
}
