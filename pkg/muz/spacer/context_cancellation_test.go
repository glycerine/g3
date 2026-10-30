package spacer

// Source: src/muz/spacer/spacer_context.cpp context::checkpoint and its
// check_reachability/propagate call sites.

import (
	"errors"
	"testing"
)

func TestSpacerQueueCancellationPrecedesClosedRootSuccess(t *testing.T) {
	for _, resourceLimit := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel-flag", true: "resource-limit"}[resourceLimit], func(t *testing.T) {

			f := newContextAnswerFixture(t, false)
			c, m := f.context, f.context.manager
			truth, err := m.MkTrue()
			if err != nil {
				t.Fatal(err)
			}
			root, err := c.transformers[f.query].NewProofObligation(nil, 0, 0, truth, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err = root.Close(); err != nil {
				t.Fatal(err)
			}
			if err = c.queue.SetRoot(root); err != nil {
				t.Fatal(err)
			}
			if resourceLimit {
				if err = m.Limit.Push(1); err != nil {
					t.Fatal(err)
				}
				defer m.Limit.Pop()
				if !m.Limit.Inc() {
					t.Fatal("initial resource allowance was unavailable")
				}
			} else {
				m.Limit.Cancel()
				defer m.Limit.ResetCancel()
			}
			before := m.Limit.Count()
			reachable, err := c.queue.CheckReachability()
			if reachable || !errors.Is(err, ErrCanceled) {
				t.Fatal("canceled queue search must stop before accepting a closed root", reachable, err)
			}
			if m.Limit.Count() != before+1 {
				t.Fatal("queue entry must take the source resource checkpoint", before, m.Limit.Count())
			}

		})
	}
}

func TestSpacerPropagationCancellationPrecedesEmptyFrameConvergence(t *testing.T) {
	f := newContextAnswerFixture(t, true)
	c, m := f.context, f.context.manager
	c.inductive = 7
	before := m.Limit.Count()
	m.Limit.Cancel()
	defer m.Limit.ResetCancel()
	converged, err := c.propagate(0, 0)
	if converged || !errors.Is(err, ErrCanceled) || c.inductive != 7 {
		t.Fatal("canceled propagation must stop before committing convergence", converged, c.inductive, err)
	}
	if m.Limit.Count() != before+1 {
		t.Fatal("propagation entry must take the source resource checkpoint", before, m.Limit.Count())
	}
}
