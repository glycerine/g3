package ast

import (
	"errors"
	"testing"
)

// Distinct owners can coexist in one mark plane. Clearing the last slot of
// one owner must preserve the other owner and leave reused slots unmarked.
func TestASTMarkOwnersWithinOnePage(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	var keep []ASTHandle
	var hs [7]ASTHandle
	for i := uint32(0); ; i++ {
		h, e := m.MkVar(i, sort)
		astTestMust(t, e)
		astTestMust(t, m.IncRef(h))
		keep = append(keep, h)
		if len(keep) >= len(hs) && (h.slot-1)%astPageSize == astPageSize-1 {
			copy(hs[:], keep[len(keep)-len(hs):])
			break
		}
	}
	const a uint64 = 1<<50 + 19
	const b uint64 = 1<<61 + 7
	for _, h := range hs[:5] {
		astTestMust(t, m.MarkNode1(h, false, a))
	}
	astTestMust(t, m.ResetNodeMark1(hs[1], a))
	astTestMust(t, m.MarkNode1(hs[5], true, b))
	astTestMust(t, m.MarkNode2(hs[0], true, b))
	if ASTSourceDebug {
		if e := m.ResetNodeMark1(hs[0], b); !errors.Is(e, ErrASTDomain) {
			t.Fatal("foreign reset accepted", e)
		}
	}
	for i, h := range hs {
		want := a
		switch i {
		case 1, 6:
			want = 0
		case 5:
			want = b
		}
		if m.nodeMarkOwner(h.slot, 1) != want {
			t.Fatalf("failed to preserve mark owner of slot %d", i)
		}
		marked, e := m.IsNodeMarked1(h, want)
		astTestMust(t, e)
		if marked != (i == 5) {
			t.Fatal("wrong mark", i)
		}
		astTestMust(t, m.ResetNodeMark1(h, want))
	}
	if set, e := m.IsNodeMarked2(hs[0], b); e != nil || !set {
		t.Fatal("second plane lost", e)
	}
	astTestMust(t, m.ResetNodeMark2(hs[0], b))
	for _, h := range hs {
		astTestMust(t, m.MarkNode1(h, false, b))
		astTestMust(t, m.ResetNodeMark1(h, b))
	}
	for _, h := range keep {
		astTestMust(t, m.DecRef(h))
	}
}
