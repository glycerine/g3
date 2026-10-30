package api

import "testing"

func TestAPIOptimizeAssertionRefsOwnResultsAfterPop(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	if err := f.opt.Push(); err != nil {
		t.Fatal(err)
	}
	if err := f.opt.Assert(f.lower); err != nil {
		t.Fatal(err)
	}
	refs, err := f.opt.AssertionRefs()
	if err != nil || len(refs) != 1 {
		t.Fatal(len(refs), err)
	}
	if err = f.opt.Pop(); err != nil {
		t.Fatal(err)
	}
	handle, err := f.m.RefGet(refs[0])
	if err != nil || handle != f.lower {
		t.Fatal(handle, err)
	}
	if err = f.m.DelRef(&refs[0]); err != nil {
		t.Fatal(err)
	}
	if _, err = f.m.RefGet(refs[0]); err == nil {
		t.Fatal("released assertion reference remained live")
	}
}
