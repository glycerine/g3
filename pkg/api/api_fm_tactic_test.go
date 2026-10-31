package api

import "testing"

func TestAPIFMTacticRegistrationApplyAndModel(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	fm, err := f.c.Tactic("fm")
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, fm.Close()) })
	sort := f.pin(f.c.RealSort())
	x := f.pin(f.c.Constant("fm-x", sort))
	lower := f.pin(f.c.Numeral("2", sort))
	upper := f.pin(f.c.Numeral("4", sort))
	g, err := f.c.NewGoal(true, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, g.Close()) })
	astTestMust(t, g.Assert(f.pin(f.c.ArithmeticGE(x, lower))))
	astTestMust(t, g.Assert(f.pin(f.c.ArithmeticLE(x, upper))))
	result, err := fm.Apply(g)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	sub, err := result.Subgoal(0)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, sub.Close()) })
	if n, err := sub.Size(); err != nil || n != 0 {
		t.Fatal("FM projection", n, err)
	}
	source, err := f.c.NewAPIModel()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, source.Close()) })
	converted, err := result.ConvertModel(0, source)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, converted.Close()) })
	decl, err := f.m.GetDecl(x)
	astTestMust(t, err)
	value, err := converted.ConstantInterpretation(decl)
	astTestMust(t, err)
	text, err := f.c.NumeralString(value)
	astTestMust(t, err)
	if text != "3" {
		t.Fatal("FM midpoint reconstruction", text)
	}
	if n, err := g.Size(); err != nil || n != 2 {
		t.Fatal("input goal", n, err)
	}
}

func TestAPIFMTacticModelConversionAtEqualSizeBoundary(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	fm, err := f.c.Tactic("fm")
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, fm.Close()) })
	real := f.pin(f.c.RealSort())
	x := f.pin(f.c.Constant("fm-index-x", real))
	goal, err := f.c.NewGoal(true, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	astTestMust(t, goal.Assert(f.pin(f.c.ArithmeticLE(x, f.pin(f.c.Numeral("4", real))))))
	result, err := fm.Apply(goal)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	count, err := result.NumSubgoals()
	astTestMust(t, err)
	source, err := f.c.NewAPIModel()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, source.Close()) })
	converted, err := result.ConvertModel(count, source)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, converted.Close()) })
	decl, err := f.m.GetDecl(x)
	astTestMust(t, err)
	value, err := converted.ConstantInterpretation(decl)
	astTestMust(t, err)
	text, err := f.c.NumeralString(value)
	astTestMust(t, err)
	if text != "3" {
		t.Fatal("source FM ignores the forwarded equal-size index", text)
	}
}
