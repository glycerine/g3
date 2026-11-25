package ast

import (
	"math"
	"testing"
)

func TestASTIncRefPairAcquiresBothOrNeither(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	h, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	node := m.nodeAt(h.slot)
	if node.rc != 0 {
		t.Fatal("fixture needs a live unreferenced node")
	}
	stale := h
	stale.generation++
	if err := m.IncRefPair(h, stale); err != ErrASTHandle || !m.IsLiveHandle(h) || node.rc != 0 {
		t.Fatalf("failed pair changed the unreferenced node: err=%v rc=%d", err, node.rc)
	}
	if ASTSourceDebug {
		node.rc = math.MaxUint32 - 1
		err := m.IncRefPair(h, h)
		count := node.rc
		node.rc = 0
		if err != ErrASTDomain || count != math.MaxUint32-1 {
			t.Fatalf("overflow on second retain: err=%v rc=%d", err, count)
		}
	}
	astTestMust(t, m.IncRefPair(h, h))
	astTestMust(t, m.IncRefPair(ASTHandle{}, h))
	astTestMust(t, m.IncRefPair(h, ASTHandle{}))
	if node.rc != 4 {
		t.Fatalf("expected four references, got %d", node.rc)
	}
	for range 4 {
		astTestMust(t, m.DecRef(h))
	}
	if m.IsLiveHandle(h) {
		t.Fatal("node survived release of all four references")
	}
}
