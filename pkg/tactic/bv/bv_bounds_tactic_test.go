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

func TestBVBoundsDominatorSourceTacticSimplifiesUsingSiblingBounds(t *testing.T) {
	f := newBVBoundsFixture(t)
	runner, e := NewBVBoundsDomTactic(f.m, f.base.context, nil)
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
		t.Fatal("dominator tactic did not simplify the implied sibling bound")
	}
}

func TestBVBoundsDominatorPropagatesSingletonEquality(t *testing.T) {
	f := newBVBoundsFixture(t)
	p := f.base.context.EmptyValue()
	defer func() { sharingMust(t, f.base.context.Del(&p)) }()
	key := "propagate_eq"
	sharingMust(t, f.base.context.SetBoolName(&p, &key, true))
	runner, e := NewBVBoundsDomTactic(f.m, f.base.context, &p)
	sharingMust(t, e)
	t.Cleanup(func() { sharingMust(t, runner.Close()) })
	ignored := f.base.context.EmptyValue()
	defer func() { sharingMust(t, f.base.context.Del(&ignored)) }()
	sharingMust(t, runner.UpdateParams(&ignored))
	translated, e := runner.Translate(f.m)
	sharingMust(t, e)
	t.Cleanup(func() { sharingMust(t, translated.(interface{ Close() error }).Close()) })
	g, e := tactic.NewGoal(f.m, false, false, false)
	sharingMust(t, e)
	t.Cleanup(func() { sharingMust(t, g.Close()) })
	lo := f.ule(t, f.num(t, 10), f.x)
	hi := f.ule(t, f.x, f.num(t, 10))
	sharingMust(t, g.Assert(lo))
	sharingMust(t, g.Assert(hi))
	var out tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, out.Close()) })
	sharingMust(t, translated.Apply(g, &out))
	form, e := g.Form(1)
	sharingMust(t, e)
	want, e := f.m.MkEq(f.x, f.num(t, 10))
	sharingMust(t, e)
	if form != want {
		t.Fatal("singleton bound was not rewritten to the equality")
	}
}

func TestBVBoundsDominatorDetectsContradictorySiblingBounds(t *testing.T) {
	f := newBVBoundsFixture(t)
	runner, e := NewBVBoundsDomTactic(f.m, f.base.context, nil)
	sharingMust(t, e)
	t.Cleanup(func() { sharingMust(t, runner.Close()) })
	g, e := tactic.NewGoal(f.m, false, false, false)
	sharingMust(t, e)
	t.Cleanup(func() { sharingMust(t, g.Close()) })
	lo := f.ule(t, f.num(t, 4), f.x)
	hi := f.ule(t, f.x, f.num(t, 2))
	sharingMust(t, g.Assert(lo))
	sharingMust(t, g.Assert(hi))
	var out tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, out.Close()) })
	sharingMust(t, runner.Apply(g, &out))
	inconsistent, e := g.Inconsistent()
	sharingMust(t, e)
	if !inconsistent {
		t.Fatal("contradictory sibling bounds did not make the goal inconsistent")
	}
}

func TestBVBoundsDominatorScopesITEBranchBounds(t *testing.T) {
	f := newBVBoundsFixture(t)
	runner, e := NewBVBoundsDomTactic(f.m, f.base.context, nil)
	sharingMust(t, e)
	t.Cleanup(func() { sharingMust(t, runner.Close()) })
	condition := f.ule(t, f.x, f.num(t, 10))
	elseBound := f.ule(t, f.num(t, 11), f.x)
	ite, e := f.m.MkIte(condition, condition, elseBound)
	sharingMust(t, e)
	f.pin(t, ite)
	g, e := tactic.NewGoal(f.m, false, false, false)
	sharingMust(t, e)
	t.Cleanup(func() { sharingMust(t, g.Close()) })
	sharingMust(t, g.Assert(ite))
	var out tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, out.Close()) })
	sharingMust(t, runner.Apply(g, &out))
	form, e := g.Form(0)
	sharingMust(t, e)
	if !f.m.IsTrue(form) {
		t.Fatalf("branch-local bounds were not applied in their respective ITE scopes: result=%v original=%v condition=%v else=%v false=%v", form, form == ite, form == condition, form == elseBound, f.m.IsFalse(form))
	}
}
