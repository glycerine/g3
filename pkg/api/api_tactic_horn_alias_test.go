package api

import "testing"

func TestAPIHornSimplifyNamedFactory(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	b := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("horn-name-p", b))
	q := f.pin(f.c.Constant("horn-name-q", b))
	rule := f.pin(f.c.Implies(p, q))
	query := f.pin(f.c.Not(q))
	g, err := f.c.NewGoal(true, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, g.Close()) }()
	astTestMust(t, g.Assert(p))
	astTestMust(t, g.Assert(rule))
	astTestMust(t, g.Assert(query))
	tac, err := f.c.Tactic("horn-simplify")
	astTestMust(t, err)
	defer func() { astTestMust(t, tac.Close()) }()
	result, err := tac.Apply(g)
	astTestMust(t, err)
	defer func() { astTestMust(t, result.Close()) }()
	count, err := result.NumSubgoals()
	astTestMust(t, err)
	if count != 1 {
		t.Fatal("subgoals", count)
	}
	goal, err := result.Subgoal(0)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	if n, err := goal.Size(); err != nil || n == 0 {
		t.Fatal("simplified goal", n, err)
	}
}

func TestAPIUFBVRewriterSourceFactoryAlias(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	b := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("alias-test-p", b))
	g, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, g.Close()) }()
	astTestMust(t, g.Assert(p))
	outputs := make([]string, 0, 2)
	for _, name := range []string{"quasi-macros", "ufbv-rewriter"} {
		tac, err := f.c.Tactic(name)
		astTestMust(t, err)
		result, err := tac.Apply(g)
		astTestMust(t, err)
		out, err := result.String()
		astTestMust(t, err)
		outputs = append(outputs, out)
		astTestMust(t, result.Close())
		astTestMust(t, tac.Close())
	}
	if outputs[0] != outputs[1] {
		t.Fatal("source registry alias mismatch", outputs)
	}
}
