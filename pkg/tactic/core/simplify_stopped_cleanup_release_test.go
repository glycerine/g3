package core

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestSimplifyCountedReleaseAfterFailedCleanupReconstruction(t *testing.T) {
	m, c, g := simplifyFixture(t)
	r, err := NewSimplifyTactic(m, c, g, nil)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, r.Close()) })
	simplifyMust(t, r.IncRef())
	m.Budget().FailAfter(0)
	err = r.Cleanup()
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || r.open() == nil {
		t.Fatal("expected a stopped, non-executable reconstruction", err)
	}
	// A source reference-counted destructor does not require an executable
	// implementation. This release must retire the real partial owner.
	simplifyMust(t, r.DecRef())
	if !r.closed {
		t.Fatal("final reference did not destroy the stopped owner")
	}
}

func TestSimplifyCleanupReconstructionCanRetry(t *testing.T) {
	success := false
	sawPartial := false
	for stop := int64(0); stop < 512 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, c, g := simplifyFixture(t)
			r, err := NewSimplifyTactic(m, c, g, nil)
			simplifyMust(t, err)
			t.Cleanup(func() { simplifyMust(t, r.Close()) })
			formula := simplifyFalseEquality(t, m)
			m.Budget().FailAfter(stop)
			err = r.Cleanup()
			m.Budget().FailAfter(-1)
			if err == nil {
				success = true
			} else {
				if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal(err)
				}
				sawPartial = sawPartial || r.rewriter != nil
				simplifyMust(t, r.Cleanup())
			}
			goal, err := tactic.NewGoal(m, false, false, false)
			simplifyMust(t, err)
			t.Cleanup(func() { simplifyMust(t, goal.Close()) })
			simplifyMust(t, goal.Assert(formula))
			var result tactic.TacticResult
			t.Cleanup(func() { simplifyMust(t, result.Close()) })
			simplifyMust(t, r.Apply(goal, &result))
			if !goal.IsDecidedUnsat() {
				t.Fatal("reconstructed simplifier did not simplify the goal")
			}
		})
	}
	if !success {
		t.Fatal("no successful cleanup reconstruction frontier")
	}
	if !sawPartial {
		t.Fatal("no partially constructed rewriter was exercised")
	}
}
