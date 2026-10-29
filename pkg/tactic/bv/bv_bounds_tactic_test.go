package bv

import (
	"github.com/glycerine/g3/pkg/tactic"
	"testing"
)

func TestBVBoundsSourceTacticSimplifiesUsingSiblingBounds(t *testing.T) {
	f := newBVBoundsFixture(t)
	runner, e := NewBVBoundsTactic(f.m, f.base.context, f.base.globals, nil, f.base.services)
	sharingMust(t, e)
	t.Cleanup(func() { sharingMust(t, runner.Close()) })
	g, e := tactic.NewGoal(f.m, false, false, false)
	sharingMust(t, e)
	t.Cleanup(func() { sharingMust(t, g.Close()) })
	lo := f.ule(t, f.num(t, 4), f.x)
	hi := f.ule(t, f.x, f.num(t, 200))
	implied := f.ule(t, f.x, f.num(t, 250))
	sharingMust(t, g.Assert(lo))
	sharingMust(t, g.Assert(hi))
	sharingMust(t, g.Assert(implied))
	var out tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, out.Close()) })
	sharingMust(t, runner.Apply(g, &out))
	last, e := g.Form(2)
	sharingMust(t, e)
	if !f.m.IsTrue(last) {
		t.Fatal("implied assertion was not simplified")
	}
}
