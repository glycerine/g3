package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
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
