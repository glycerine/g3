package bv

import (
	"github.com/glycerine/g3/pkg/tactic"
	"testing"
)

func TestBVSliceSourceNoEqualityIsIdentity(t *testing.T) {
	f := newBVSliceFixture(t)
	g, e := tactic.NewGoal(f.m, false, false, false)
	sharingMust(t, e)
	t.Cleanup(func() { sharingMust(t, g.Close()) })
	sum, e := f.m.MkBitVectorAdd(f.x, f.y)
	sharingMust(t, e)
	formula, e := f.m.MkEq(sum, f.x)
	sharingMust(t, e)
	sharingMust(t, g.Assert(formula))
	var out tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, out.Close()) })
	sharingMust(t, f.tactic.Apply(g, &out))
	got, e := g.Form(0)
	sharingMust(t, e)
	if got != formula {
		t.Fatal("unnecessary rewrite")
	}
}
