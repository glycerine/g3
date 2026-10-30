package spacer

// Source: src/muz/spacer/spacer_context.cpp context::expand_node's
// pdr_flexible_trace / is_blocked fast path.

import (
	"testing"
)

func TestSpacerFlexibleTraceSkipsExpansionOfFrameBlockedRoot(t *testing.T) {
	for _, flexible := range []bool{false, true} {
		name := "ordinary-expansion"
		if flexible {
			name = "frame-blocked"
		}
		t.Run(name, func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			c, m := f.context, f.context.manager
			p := c.transformers[f.query]
			boolean, err := m.MkBoolSort()
			depthMust(t, err)
			a, err := m.MkFreshConst("flexible-blocker", boolean)
			depthMust(t, err)
			pin, err := m.NewRef(a)
			depthMust(t, err)
			defer func() { depthMust(t, m.DelRef(&pin)) }()
			_, err = p.AddLemma(a, InfinityLevel)
			depthMust(t, err)
			post, err := m.MkNot(a)
			depthMust(t, err)
			root, err := p.NewProofObligation(nil, 0, 0, post, nil)
			depthMust(t, err)
			depthMust(t, c.queue.SetRoot(root))
			before := p.frames.additions
			reachable, err := c.queue.checkReachability(true, flexible, InfinityLevel, false, 0, nil)
			depthMust(t, err)
			if reachable || root.level != 1 {
				t.Fatal("expected blocked root promoted to level one", reachable, root.level)
			}
			expected := uint32(0)
			if flexible {
				expected = InfinityLevel
			}
			if c.queue.ExpandedLevel() != expected {
				t.Fatalf("expanded level = %d, want %d", c.queue.ExpandedLevel(), expected)
			}
			if flexible {
				if p.frames.additions != before {
					t.Fatal("frame-blocked root generated a lemma")
				}
				if len(c.queue.items) != 1 || c.queue.items[0] != root {
					t.Fatal("flexible policy did not requeue blocked root")
				}
				current, err := root.Post()
				depthMust(t, err)
				if current != post {
					t.Fatal("frame-blocked root changed its postcondition")
				}
			}
		})
	}
}

func TestSpacerFlexibleTraceStillExpandsReachableRoot(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	c := f.context
	truth, err := c.manager.MkTrue()
	depthMust(t, err)
	root, err := c.transformers[f.query].NewProofObligation(nil, 0, 0, truth, nil)
	depthMust(t, err)
	depthMust(t, c.queue.SetRoot(root))
	reachable, err := c.queue.checkReachability(true, true, InfinityLevel, false, 0, nil)
	depthMust(t, err)
	if !reachable {
		t.Fatal("flexible precheck suppressed reachable expansion")
	}
	closed, err := root.IsClosed()
	depthMust(t, err)
	if !closed {
		t.Fatal("reachable root was not closed")
	}
}
