//go:build g3debug

package ast

import (
	"errors"
	"math"
	"testing"
)

func TestASTSharedOccurrencePublishesBeforeRefcountAssertion(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 1)[0]
	astTestMust(t, m.IncRef(h))
	s, err := NewASTSharedOccurrencesWithConfig(m, ASTSharedOccurrencesConfig{TrackAtomic: true})
	astTestMust(t, err)
	astTestMust(t, m.lock())
	r, err := m.internal(h)
	astTestMust(t, err)
	n := m.nodeAt(r.slot)
	oldRC, oldShared := n.rc, n.hasFlag(astShared)
	n.rc = math.MaxUint32
	n.flags |= astShared
	var marks astSharedMarkState
	_, err = s.process(m, r, &marks)
	n.rc = oldRC
	n.setFlag(astShared, oldShared)
	m.mu.Unlock()
	if !errors.Is(err, ErrASTDomain) {
		t.Fatalf("source INC assertion after set publication: %v", err)
	}
	entry, found := s.entries.Get2(astSharedKey(r))
	if !found || entry.node != r || entry.owned || s.NumShared() != 1 {
		t.Fatalf("stopped source set entry = %+v, found=%v, size=%d", entry, found, s.NumShared())
	}
	astTestMust(t, s.Reset())
	if s.NumShared() != 0 {
		t.Fatalf("owning reset retained unacquired source entry: %d", s.NumShared())
	}
	if rc, err := m.GetRefCount(h); err != nil || rc != oldRC {
		t.Fatalf("refcount after unacquired entry cleanup = %d, %v", rc, err)
	}
	astTestMust(t, s.Close())
	astTestMust(t, m.DecRef(h))
}
