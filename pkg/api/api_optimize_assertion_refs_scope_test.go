package api

import "testing"

func TestAPIOptimizeAssertionRefsFollowScopedInsertionOrder(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	if err := f.opt.Assert(f.lower); err != nil {
		t.Fatal(err)
	}
	if err := f.opt.Push(); err != nil {
		t.Fatal(err)
	}
	if err := f.opt.Assert(f.upper); err != nil {
		t.Fatal(err)
	}
	refs, err := f.opt.AssertionRefs()
	if err != nil || len(refs) != 2 {
		t.Fatal(len(refs), err)
	}
	first, err := f.m.RefGet(refs[0])
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.m.RefGet(refs[1])
	if err != nil {
		t.Fatal(err)
	}
	if first != f.lower || second != f.upper {
		t.Fatal(first, second)
	}
	for i := range refs {
		if err = f.m.DelRef(&refs[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err = f.opt.Pop(); err != nil {
		t.Fatal(err)
	}
	refs, err = f.opt.AssertionRefs()
	if err != nil || len(refs) != 1 {
		t.Fatal(len(refs), err)
	}
	if err = f.m.DelRef(&refs[0]); err != nil {
		t.Fatal(err)
	}
}
