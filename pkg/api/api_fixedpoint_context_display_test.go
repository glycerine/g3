package api

import (
	"errors"
	"strings"
	"testing"
)

func TestSMTContextFixedpointSourceDisplayAndExplicitReplay(t *testing.T) {
	c, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, c.Close()) }()
	m, err := c.Manager()
	astTestMust(t, err)
	p, err := c.DeclareRelation("context-display-p", nil)
	astTestMust(t, err)
	q, err := c.DeclareRelation("context-display-q", nil)
	astTestMust(t, err)
	_, err = c.DeclareRelation("context-display-unused", nil)
	astTestMust(t, err)
	pa, err := m.MkApp(p, nil)
	astTestMust(t, err)
	qa, err := m.MkApp(q, nil)
	astTestMust(t, err)
	step, err := m.MkImplies(pa, qa)
	astTestMust(t, err)
	astTestMust(t, c.AddFixedpointFact(p, nil))
	astTestMust(t, c.AddFixedpointRule(step, "context-step"))
	// AddRule only queues (dl_context::add_rule); the first reader builds and
	// keeps the Go eager rule view, so prime it before counting references.
	if _, err := c.FixedpointSMT2(qa, pa); err != nil {
		t.Fatal(err)
	}
	before := m.Stats().LiveReferences
	source, err := c.FixedpointSMT2(qa, pa)
	astTestMust(t, err)
	if strings.Contains(source, "context-display-unused") {
		t.Fatal("source display included unused registration", source)
	}
	qi := strings.Index(source, "(query context-display-q)")
	pi := strings.Index(source, "(query context-display-p)")
	if qi < 0 || pi <= qi {
		t.Fatal("source display lost ordered queries", source)
	}
	var written strings.Builder
	astTestMust(t, c.WriteFixedpointSMT2(&written, qa, pa))
	if written.String() != source {
		t.Fatal("source writer/string differ", source, written.String())
	}
	if m.Stats().LiveReferences != before {
		t.Fatal("source display leaked references")
	}
	astTestMust(t, c.SetBoolOption("fixedpoint.print_fixedpoint_extensions", false))
	plain, err := c.FixedpointSMT2(qa)
	astTestMust(t, err)
	if !strings.HasPrefix(plain, "(set-logic HORN)") || !strings.Contains(plain, "(check-sat)") || strings.Contains(plain, "declare-rel") {
		t.Fatal("legacy context ignored source print profile", plain)
	}
	replay, err := c.FixedpointReplaySMT2()
	astTestMust(t, err)
	if !strings.Contains(replay, "(declare-rel context-display-unused ())") || strings.Contains(replay, "check-sat") {
		t.Fatal("explicit replay changed with source profile", replay)
	}
	written.Reset()
	astTestMust(t, c.WriteFixedpointReplaySMT2(&written))
	if replay != written.String() {
		t.Fatal("replay writer/string differ")
	}
	truth, err := m.MkTrue()
	astTestMust(t, err)
	astTestMust(t, c.AssertFixedpoint(truth))
	if text, err := c.FixedpointSMT2(); err == nil || text != "" {
		t.Fatal("plain source background rejection lost", text, err)
	}
	replay, err = c.FixedpointReplaySMT2()
	astTestMust(t, err)
	if !strings.Contains(replay, "(assert true)") {
		t.Fatal("replay lost background", replay)
	}
}

func TestSMTContextFixedpointDisplayAndReplayLifecycleGuards(t *testing.T) {
	c, err := NewSMTContext()
	astTestMust(t, err)
	if err := c.WriteFixedpointSMT2(nil); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("nil source writer", err)
	}
	if err := c.WriteFixedpointReplaySMT2(nil); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("nil replay writer", err)
	}
	astTestMust(t, c.Close())
	if _, err := c.FixedpointSMT2(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed source display", err)
	}
	if _, err := c.FixedpointReplaySMT2(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed replay display", err)
	}
}
