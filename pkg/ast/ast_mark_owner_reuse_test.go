package ast

import (
	"errors"
	"math"
	"testing"
)

// A false bit may still have an owner. Both planes accept full-width owner
// identities, and a later generation at the exact slot starts unowned.
func TestNodeMarkOwnersAcrossPagesAndSlotReuse(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	var vars [2*astPageSize + 7]ASTHandle
	for i := range vars {
		vars[i], err = m.MkVar(uint32(i), sort)
		astTestMust(t, err)
		astTestMust(t, m.IncRef(vars[i]))
	}
	first, last := vars[0], vars[len(vars)-1]
	if (first.slot-1)/astPageSize == (last.slot-1)/astPageSize {
		t.Fatal("fixture needs different pages")
	}
	const owner1 uint64 = math.MaxUint64
	const owner2 uint64 = (1 << 32) + 1
	astTestMust(t, m.MarkNode1(first, false, owner1))
	astTestMust(t, m.MarkNode2(first, true, owner2))
	astTestMust(t, m.MarkNode1(last, true, owner2))
	astTestMust(t, m.MarkNode2(last, false, owner1))
	if m.nodeMarkOwner(first.slot, 1) != owner1 || m.nodeMarkOwner(first.slot, 2) != owner2 ||
		m.nodeMarkOwner(last.slot, 1) != owner2 || m.nodeMarkOwner(last.slot, 2) != owner1 {
		t.Fatal("mark owner truncated or copied across planes")
	}
	if ASTSourceDebug {
		if err := m.ResetNodeMark1(first, owner2); !errors.Is(err, ErrASTDomain) {
			t.Fatalf("wrong owner reset a false mark: %v", err)
		}
	}
	astTestMust(t, m.ResetNodeMark1(first, owner1))
	if marked, e := m.IsNodeMarked2(first, owner2); e != nil || !marked {
		t.Fatal("one plane reset the other", e)
	}
	if m.nodeMarkOwner(first.slot, 1) != 0 || m.nodeMarkOwner(first.slot, 2) != owner2 {
		t.Fatal("incorrect reset")
	}
	astTestMust(t, m.DecRef(last))
	replacement, err := m.MkVar(100000, sort)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(replacement))
	if replacement.slot != last.slot || replacement.generation == last.generation {
		t.Fatal("fixture did not reuse the marked slot")
	}
	if m.nodeMarkOwner(replacement.slot, 1) != 0 || m.nodeMarkOwner(replacement.slot, 2) != 0 {
		t.Fatal("new generation inherited mark owners")
	}
	for _, read := range []func(ASTHandle, uint64) (bool, error){m.IsNodeMarked1, m.IsNodeMarked2} {
		bit, e := read(replacement, owner1)
		if e != nil || bit {
			t.Fatal("reused mark", bit, e)
		}
		if _, e = read(last, owner1); !errors.Is(e, ErrASTHandle) {
			t.Fatal("stale mark token", e)
		}
	}
	astTestMust(t, m.ResetNodeMark2(first, owner2))
	astTestMust(t, m.DecRef(replacement))
	for _, h := range vars[:len(vars)-1] {
		astTestMust(t, m.DecRef(h))
	}
}
