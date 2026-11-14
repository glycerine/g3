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
	truth, e := f.m.MkTrue()
	domMust(t, e)
	then, e := f.m.MkAnd(truth, f.b)
	domMust(t, e)
	want, e := f.m.MkIte(f.a, then, no)
	domMust(t, e)
	if got != want {
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
	// simplify_ite tests c == new_c && t == new_t && e == new_e before
	// new_t == new_e, so an unchanged (ite a b b) is kept (z3 prints it).
	if got != ite {
		t.Fatal(got, ite)
	}
}
