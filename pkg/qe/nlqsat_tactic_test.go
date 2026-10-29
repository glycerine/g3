package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	basetactic "github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestNLQSATTacticClosedExistentialAndFalseGoals(t *testing.T) {
	t.Run("existential", func(t *testing.T) {
		fixture := qsatPrepareFixture(t, "exists")
		f := fixture.f
		tactic, err := NewNLQSATTactic(f.r.manager, f.r.config)
		fmMust(t, err)
		t.Cleanup(func() { fmMust(t, tactic.Close()) })
		goal := f.goal(false, false)
		result := f.result()
		fmMust(t, goal.Assert(fixture.original))
		fmMust(t, tactic.Apply(goal, result))
		size, err := goal.Size()
		fmMust(t, err)
		if size != 0 || len(result.Goals) != 1 || result.Goals[0] != goal {
			t.Fatal("existential goal was not decided SAT", size, len(result.Goals))
		}
		if result.ModelConverter == nil {
			t.Fatal("existential SAT model converter was not published")
		}
		var model basetactic.ModelRef
		t.Cleanup(func() { fmMust(t, model.Close()) })
		fmMust(t, result.ModelConverter.ApplyUnindexed(&model))
		if core, cause := model.Get(); cause != nil || core == nil {
			t.Fatal("published model is unavailable", core, cause)
		}
		fmMust(t, tactic.Close())
		fmMust(t, result.ModelConverter.ApplyUnindexed(&model))
	})
	t.Run("false", func(t *testing.T) {
		f := newFMFixture(t)
		tactic, err := NewNLQSATTactic(f.r.manager, f.r.config)
		fmMust(t, err)
		t.Cleanup(func() { fmMust(t, tactic.Close()) })
		goal := f.goal(false, false)
		result := f.result()
		falsity, err := f.r.manager.MkFalse()
		fmMust(t, err)
		fmMust(t, goal.Assert(falsity))
		fmMust(t, tactic.Apply(goal, result))
		if !goal.IsDecidedUnsat() || len(result.Goals) != 1 {
			t.Fatal("false goal was not decided UNSAT", goal.IsDecidedUnsat(), len(result.Goals))
		}
	})
}

func TestNLQSATTacticQuantifiedNonlinearRealGoals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		forall bool
		body   func(*fmFixture, ast.ASTHandle) ast.ASTHandle
		sat    bool
	}{
		{name: "exists-sqrt-two", body: func(f *fmFixture, x ast.ASTHandle) ast.ASTHandle {
			square := f.own(f.r.manager.MkArithmeticMul(x, x))
			two := f.own(f.r.manager.ArithmeticRealNumeralText("2"))
			return f.own(f.r.manager.MkEq(square, two))
		}, sat: true},
		{name: "forall-square-nonnegative", forall: true, body: func(f *fmFixture, x ast.ASTHandle) ast.ASTHandle {
			square := f.own(f.r.manager.MkArithmeticMul(x, x))
			zero := f.own(f.r.manager.ArithmeticRealNumeralText("0"))
			return f.own(f.r.manager.MkArithmeticGE(square, zero))
		}, sat: true},
		{name: "forall-nonpositive", forall: true, body: func(f *fmFixture, x ast.ASTHandle) ast.ASTHandle {
			zero := f.own(f.r.manager.ArithmeticRealNumeralText("0"))
			return f.own(f.r.manager.MkArithmeticLE(x, zero))
		}, sat: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFMFixture(t)
			x := f.variable(0, f.real)
			name, err := f.r.manager.Symbols.Intern("x")
			fmMust(t, err)
			formula := f.own(f.r.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: tc.forall, Sorts: []ast.ASTHandle{f.real}, Names: []usymbol.Symbol{name}, Body: tc.body(f, x)}))
			tactic, err := NewNLQSATTactic(f.r.manager, f.r.config)
			fmMust(t, err)
			t.Cleanup(func() { fmMust(t, tactic.Close()) })
			goal := f.goal(false, false)
			result := f.result()
			fmMust(t, goal.Assert(formula))
			fmMust(t, tactic.Apply(goal, result))
			if tc.sat {
				size, cause := goal.Size()
				fmMust(t, cause)
				if size != 0 {
					t.Fatal("quantified nonlinear goal was not decided SAT", size)
				}
			} else if !goal.IsDecidedUnsat() {
				t.Fatal("quantified nonlinear goal was not decided UNSAT")
			}
		})
	}
}

func TestNLQSATTacticMixedRealAlternationAndProjection(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		outerForall, innerForall bool
		body                     func(*fmFixture, ast.ASTHandle, ast.ASTHandle) ast.ASTHandle
		sat                      bool
	}{
		{name: "exists-forall-unbounded", innerForall: true, body: func(f *fmFixture, x, y ast.ASTHandle) ast.ASTHandle {
			return f.own(f.r.manager.MkArithmeticGE(x, y))
		}, sat: false},
		{name: "forall-exists-larger", outerForall: true, body: func(f *fmFixture, x, y ast.ASTHandle) ast.ASTHandle {
			atMost := f.own(f.r.manager.MkArithmeticLE(y, x))
			return f.own(f.r.manager.MkNot(atMost))
		}, sat: true},
		{name: "exists-forall-square-bound", innerForall: true, body: func(f *fmFixture, x, y ast.ASTHandle) ast.ASTHandle {
			square := f.own(f.r.manager.MkArithmeticMul(y, y))
			return f.own(f.r.manager.MkArithmeticLE(x, square))
		}, sat: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFMFixture(t)
			y, x := f.variable(0, f.real), f.variable(1, f.real)
			xName, err := f.r.manager.Symbols.Intern("x")
			fmMust(t, err)
			yName, err := f.r.manager.Symbols.Intern("y")
			fmMust(t, err)
			inner := f.own(f.r.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: tc.innerForall, Sorts: []ast.ASTHandle{f.real}, Names: []usymbol.Symbol{yName}, Body: tc.body(f, x, y)}))
			formula := f.own(f.r.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: tc.outerForall, Sorts: []ast.ASTHandle{f.real}, Names: []usymbol.Symbol{xName}, Body: inner}))
			tactic, err := NewNLQSATTactic(f.r.manager, f.r.config)
			fmMust(t, err)
			t.Cleanup(func() { fmMust(t, tactic.Close()) })
			goal := f.goal(false, false)
			result := f.result()
			fmMust(t, goal.Assert(formula))
			fmMust(t, tactic.Apply(goal, result))
			if tc.sat {
				size, cause := goal.Size()
				fmMust(t, cause)
				if size != 0 {
					t.Fatal("mixed quantified goal was not decided SAT", size)
				}
			} else if !goal.IsDecidedUnsat() {
				t.Fatal("mixed quantified goal was not decided UNSAT")
			}
			if rounds := tactic.CollectStatistics()["qsat num rounds"]; rounds < 3 {
				t.Fatal("mixed formula did not exercise projected alternation", rounds)
			}
		})
	}
}

func TestNLQSATTacticPublishesOnlyFreeRealModelValues(t *testing.T) {
	f := newFMFixture(t)
	free := f.constant("nlq-free-real", f.real)
	x := f.variable(0, f.real)
	equality := f.own(f.r.manager.MkEq(x, free))
	name, err := f.r.manager.Symbols.Intern("bound-x")
	fmMust(t, err)
	formula := f.own(f.r.manager.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.real}, Names: []usymbol.Symbol{name}, Body: equality}))
	runner, err := NewNLQSATTactic(f.r.manager, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, runner.Close()) })
	goal := f.goal(false, false)
	result := f.result()
	fmMust(t, goal.Assert(formula))
	fmMust(t, runner.Apply(goal, result))
	if result.ModelConverter == nil {
		t.Fatal("free-real model converter was not published")
	}
	var output basetactic.ModelRef
	t.Cleanup(func() { fmMust(t, output.Close()) })
	fmMust(t, result.ModelConverter.ApplyUnindexed(&output))
	model, err := output.Get()
	fmMust(t, err)
	declaration, err := f.r.manager.GetDecl(free)
	fmMust(t, err)
	if value, cause := model.ConstantInterpretation(declaration); cause != nil || value.IsNull() {
		t.Fatal("free real has no model value", value, cause)
	}
}
