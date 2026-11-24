package ast

import "testing"

func TestReleaseSharedAppSuffixStopsAtLastOwner(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	mk := func(name string) ASTHandle {
		h, e := m.MkConst(astTestSymbol(t, m, name), sort)
		astTestMust(t, e)
		astTestMust(t, m.IncRef(h))
		return h
	}
	left, stop, right := mk("left"), mk("last-owner"), mk("right")
	astTestMust(t, m.IncRef(left))
	astTestMust(t, m.IncRef(right))
	astTestMust(t, m.IncRef(right))
	// The repeated right handle can lose two references. The stop and
	// everything before it must still have exactly their original owners.
	n := m.ReleaseSharedAppSuffix([]ASTHandle{left, stop, right, right})
	want := 2
	if ASTValidateHandles {
		want = 0
	}
	if n != want {
		t.Fatal("consumed", n, "want", want)
	}
	for _, tc := range []struct {
		h    ASTHandle
		want uint32
	}{{left, 2}, {stop, 1}, {right, uint32(3 - want)}} {
		got, e := m.GetRefCount(tc.h)
		astTestMust(t, e)
		if got != tc.want {
			t.Fatal("reference count", got, tc.want)
		}
	}
	other := astTestManager(t)
	if other.ReleaseSharedAppSuffix([]ASTHandle{left}) != 0 || m.ReleaseSharedAppSuffix([]ASTHandle{{}}) != 0 || m.ReleaseSharedAppSuffix([]ASTHandle{sort}) != 0 {
		t.Fatal("consumed foreign, null or sort reference")
	}
	for _, h := range []ASTHandle{left, left, stop} {
		astTestMust(t, m.DecRef(h))
	}
	for i := 0; i < 3-want; i++ {
		astTestMust(t, m.DecRef(right))
	}
	if m.ReleaseSharedAppSuffix([]ASTHandle{left}) != 0 {
		t.Fatal("consumed retired handle")
	}
}
