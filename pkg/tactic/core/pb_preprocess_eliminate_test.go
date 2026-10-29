package core

import "testing"

func TestPBPreprocessEliminatesOneSidedVariables(t *testing.T) {
	f := newPBPreprocessFixture(t)
	h := f.ge(f.vars[:2], []string{"1", "1"}, "1")
	g := f.goal(h)
	out := f.apply(g)
	if len(out.Goals) != 1 || out.ModelConverter == nil {
		t.Fatal("missing result/converter")
	}
	if n, e := g.Size(); e != nil || n != 0 {
		t.Fatal("one-sided constraints survived", n, e)
	}
	if d, e := g.Depth(); e != nil || d != 1 {
		t.Fatal(d, e)
	}
}

func TestPBPreprocessNegativeLiteralChoosesFalse(t *testing.T) {
	f := newPBPreprocessFixture(t)
	notX, e := f.m.MkNot(f.vars[0])
	simplifyMust(t, e)
	g := f.goal(notX)
	f.apply(g)
	if n, e := g.Size(); e != nil || n != 0 {
		t.Fatal(n, e)
	}
}
