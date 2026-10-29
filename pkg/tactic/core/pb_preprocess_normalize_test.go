package core

import "testing"

func TestPBPreprocessNormalizesNegatedGE(t *testing.T) {
	f := newPBPreprocessFixture(t)
	ge := f.ge(f.vars[:2], []string{"2", "3"}, "4")
	negated, e := f.m.MkNot(ge)
	simplifyMust(t, e)
	g := f.goal(negated)
	f.apply(g)
	if n, e := g.Size(); e != nil || n != 0 {
		t.Fatal("normalized one-sided constraint survived", n, e)
	}
}
