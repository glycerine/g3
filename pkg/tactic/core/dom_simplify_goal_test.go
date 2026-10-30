package core

import "testing"

func TestDomSimplifyUsesEarlierAndLaterGoalFormulas(t *testing.T) {
	f := newDomFixture(t)
	na, e := f.m.MkNot(f.a)
	domMust(t, e)
	disj, e := f.m.MkOr(na, f.b)
	domMust(t, e)
	g := f.goal(t, false, f.a, disj)
	f.run(t, g)
	second, e := g.Form(1)
	domMust(t, e)
	if second != disj {
		t.Fatal(second, disj)
	}
	depth, e := g.Depth()
	domMust(t, e)
	if depth != 1 {
		t.Fatal(depth)
	}
}
func TestDomSimplifyEqualitySubstitution(t *testing.T) {
	f := newDomFixture(t)
	eq, e := f.m.MkEq(f.a, f.b)
	domMust(t, e)
	use, e := f.m.MkEq(f.a, f.b)
	domMust(t, e)
	g := f.goal(t, false, eq, use)
	f.run(t, g)
	want, e := f.m.MkEq(f.a, f.a)
	domMust(t, e)
	got, e := g.Form(1)
	domMust(t, e)
	if got != want {
		t.Fatal(got, want)
	}
}

func TestDomSimplifyComposesRewriteProof(t *testing.T) {
	f := newDomFixture(t)
	na, e := f.m.MkNot(f.a)
	domMust(t, e)
	tail, e := f.m.MkOr(na, f.b)
	domMust(t, e)
	formula, e := f.m.MkAnd(f.a, tail)
	domMust(t, e)
	g := f.goal(t, true, formula)
	f.run(t, g)
	n, e := g.Size()
	domMust(t, e)
	if n != 2 {
		t.Fatal(n)
	}
	for i := uint32(0); i < n; i++ {
		proof, e := g.Proof(i)
		domMust(t, e)
		if proof.IsNull() {
			t.Fatal("missing composed proof", i)
		}
	}
}
