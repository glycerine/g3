package core

import "testing"

func TestDomSimplifyITEBranchContexts(t *testing.T) {
	f := newDomFixture(t)
	na, e := f.m.MkNot(f.a)
	domMust(t, e)
	yes, e := f.m.MkAnd(f.a, f.b)
	domMust(t, e)
	no, e := f.m.MkAnd(na, f.b)
	domMust(t, e)
	ite, e := f.m.MkIte(f.a, yes, no)
	domMust(t, e)
	g := f.goal(t, false, ite)
	f.run(t, g)
	got, e := g.Form(0)
	domMust(t, e)
	want, e := f.m.MkIte(f.a, f.b, f.b)
	domMust(t, e)
	if got != f.b && got != want {
		t.Fatal(got, want)
	}
}
func TestDomSimplifyITEEqualBranches(t *testing.T) {
	f := newDomFixture(t)
	ite, e := f.m.MkIte(f.a, f.b, f.b)
	domMust(t, e)
	g := f.goal(t, false, ite)
	f.run(t, g)
	got, e := g.Form(0)
	domMust(t, e)
	if got != f.b {
		t.Fatal(got, f.b)
	}
}
