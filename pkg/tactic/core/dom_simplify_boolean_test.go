package core

import "testing"

func TestDomSimplifyForwardConjunctionContext(t *testing.T) {
	f := newDomFixture(t)
	na, e := f.m.MkNot(f.a)
	domMust(t, e)
	tail, e := f.m.MkOr(na, f.b)
	domMust(t, e)
	formula, e := f.m.MkAnd(f.a, tail)
	domMust(t, e)
	g := f.goal(t, false, formula)
	f.run(t, g)
	n, e := g.Size()
	domMust(t, e)
	if n != 2 {
		t.Fatal("goal conjunction publication", n)
	}
	first, e := g.Form(0)
	domMust(t, e)
	second, e := g.Form(1)
	domMust(t, e)
	if first != f.a || second != tail {
		t.Fatal(first, second, f.a, tail)
	}
}
func TestDomSimplifyDisjunctionContext(t *testing.T) {
	f := newDomFixture(t)
	na, e := f.m.MkNot(f.a)
	domMust(t, e)
	tail, e := f.m.MkAnd(na, f.b)
	domMust(t, e)
	formula, e := f.m.MkOr(f.a, tail)
	domMust(t, e)
	g := f.goal(t, false, formula)
	f.run(t, g)
	got, e := g.Form(0)
	domMust(t, e)
	if got != formula {
		t.Fatal(got, formula)
	}
}
