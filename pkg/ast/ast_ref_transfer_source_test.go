package ast

// Source: src/util/obj_ref.h:56-60,100-110; src/ast/ast.h:2258-2264.

import (
	"testing"
)

func TestASTRefMoveSwapStealSourceOwnership(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkVar(1901, sort)
	astTestMust(t, err)
	b, err := m.MkVar(1902, sort)
	astTestMust(t, err)
	ra, err := m.NewRef(a)
	astTestMust(t, err)
	rb, err := m.NewRef(b)
	astTestMust(t, err)
	check := func(h ASTHandle, want uint32) {
		t.Helper()
		got, e := m.GetRefCount(h)
		astTestMust(t, e)
		if got != want {
			t.Fatalf("reference count for %v: got %d, want %d", h, got, want)
		}
	}
	check(a, 1)
	check(b, 1)
	moved, err := m.MoveRef(ra)
	astTestMust(t, err)
	if old, e := m.RefGet(ra); e != nil || !old.IsNull() {
		t.Fatalf("moved-from reference: %v, %v", old, e)
	}
	check(a, 1)
	m.BudgetField.FailAfter(0)
	astTestMust(t, m.SwapRefs(moved, rb))
	if got, e := m.RefGet(moved); e != nil || got != b {
		t.Fatalf("swapped first reference: %v, %v", got, e)
	}
	if got, e := m.RefGet(rb); e != nil || got != a {
		t.Fatalf("swapped second reference: %v, %v", got, e)
	}
	stolen, err := m.StealRef(moved)
	astTestMust(t, err)
	if stolen != b {
		t.Fatalf("stolen pointer: %v, want %v", stolen, b)
	}
	if old, e := m.RefGet(moved); e != nil || !old.IsNull() {
		t.Fatalf("stolen-from reference: %v, %v", old, e)
	}
	astTestMust(t, m.DelRef(&moved))
	astTestMust(t, m.DelRef(&ra))
	check(a, 1)
	check(b, 1)
	m.BudgetField.FailAfter(-1)
	astTestMust(t, m.DelRef(&rb))
	astTestMust(t, m.DecRef(stolen))
	if m.IsLiveHandle(a) {
		t.Fatal("owned first node survived reference destruction")
	}
	if m.IsLiveHandle(b) {
		t.Fatal("stolen node survived explicit release")
	}
}
