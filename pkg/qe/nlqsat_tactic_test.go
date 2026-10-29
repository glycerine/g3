package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
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

func TestNLQSATTacticPurifiesSymbolicDivision(t *testing.T) {
	for _, consistency := range []bool{false, true} {
		t.Run(map[bool]string{false: "satisfiable", true: "functional-consistency"}[consistency], func(t *testing.T) {
			f := newFMFixture(t)
			a := f.constant("division-a", f.real)
			b := f.constant("division-b", f.real)
			c, d := a, b
			if consistency {
				c = f.constant("division-c", f.real)
				d = f.constant("division-d", f.real)
			}
			zero := f.own(f.r.manager.ArithmeticRealNumeralText("0"))
			two := f.own(f.r.manager.ArithmeticRealNumeralText("2"))
			bZero := f.own(f.r.manager.MkEq(b, zero))
			bNonzero := f.own(f.r.manager.MkNot(bZero))
			left := f.own(f.r.manager.MkArithmeticDiv(a, b))
			var formula ast.ASTHandle
			if consistency {
				right := f.own(f.r.manager.MkArithmeticDiv(c, d))
				aEqual := f.own(f.r.manager.MkEq(a, c))
				bEqual := f.own(f.r.manager.MkEq(b, d))
				valuesEqual := f.own(f.r.manager.MkEq(left, right))
				valuesDiffer := f.own(f.r.manager.MkNot(valuesEqual))
				formula = f.own(f.r.manager.MkAndArray([]ast.ASTHandle{bNonzero, aEqual, bEqual, valuesDiffer}))
			} else {
				value := f.own(f.r.manager.MkEq(left, two))
				formula = f.own(f.r.manager.MkAnd(bNonzero, value))
			}
			runner, err := NewNLQSATTactic(f.r.manager, f.r.config)
			fmMust(t, err)
			t.Cleanup(func() { fmMust(t, runner.Close()) })
			goal := f.goal(false, false)
			result := f.result()
			fmMust(t, goal.Assert(formula))
			fmMust(t, runner.Apply(goal, result))
			if consistency {
				if !goal.IsDecidedUnsat() {
					t.Fatal("division congruence axiom was not enforced")
				}
			} else if size, cause := goal.Size(); cause != nil || size != 0 {
				t.Fatal("symbolic division goal was not decided SAT", size, cause)
			}
		})
	}
}

func TestNLQSATTacticParametersAndTranslation(t *testing.T) {
	f := newFMFixture(t)
	runner, err := NewNLQSATTactic(f.r.manager, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, runner.Close()) })
	params, err := f.r.config.Context.NewValue()
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, f.r.config.Context.Del(&params)) })
	seedName, seed := "seed", uint32(19)
	fmMust(t, f.r.config.Context.SetUIntName(&params, &seedName, seed))
	randomizeName, disabled := "randomize", false
	fmMust(t, f.r.config.Context.SetBoolName(&params, &randomizeName, disabled))
	factorName, enabled := "factor", true
	fmMust(t, f.r.config.Context.SetBoolName(&params, &factorName, enabled))
	fmMust(t, runner.UpdateParams(&params))
	translated, err := runner.Translate(f.r.manager)
	fmMust(t, err)
	copy, ok := translated.(*NLQSATTactic)
	if !ok {
		t.Fatal("translated tactic type")
	}
	t.Cleanup(func() { fmMust(t, copy.Close()) })
	if got, cause := f.r.config.Context.GetUIntName(&copy.params, &seedName, 0); cause != nil || got != seed {
		t.Fatal("translated seed", got, cause)
	}
	bare := nlqBareSolverWithFixture(t, f)
	polynomials, err := bare.PolynomialManager()
	fmMust(t, err)
	algebraic, err := bare.AlgebraicManager()
	fmMust(t, err)
	configuration, err := copy.solverConfig(polynomials, algebraic)
	fmMust(t, err)
	if configuration.Seed != seed || configuration.Randomize || configuration.Factor {
		t.Fatal("NLQSAT parameter projection", configuration.Seed, configuration.Randomize, configuration.Factor)
	}
	fmMust(t, runner.UpdateParams(nil))
	if got, cause := f.r.config.Context.GetUIntName(&copy.params, &seedName, 0); cause != nil || got != seed {
		t.Fatal("translation did not retain an independent parameter snapshot", got, cause)
	}
}

func TestNLQETacticEliminatesNonlinearExistential(t *testing.T) {
	f := newFMFixture(t)
	k := f.constant("nlqe-k", f.real)
	x := f.variable(0, f.real)
	square := f.own(f.r.manager.MkArithmeticMul(x, x))
	body := f.own(f.r.manager.MkEq(square, k))
	name, err := f.r.manager.Symbols.Intern("nlqe-x")
	fmMust(t, err)
	formula := f.own(f.r.manager.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.real}, Names: []usymbol.Symbol{name}, Body: body}))
	eliminate, err := NewNLQETactic(f.r.manager, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, eliminate.Close()) })
	goal := f.goal(false, false)
	result := f.result()
	fmMust(t, goal.Assert(formula))
	fmMust(t, eliminate.Apply(goal, result))
	if size, cause := goal.Size(); cause != nil || size != 1 {
		t.Fatal("nonlinear QE result size", size, cause)
	}
	projected, err := goal.Form(0)
	fmMust(t, err)
	if quantified, cause := f.r.manager.HasQuantifiers(projected); cause != nil || quantified {
		t.Fatal("nonlinear QE retained a quantifier", quantified, cause)
	}
	for _, tc := range []struct {
		value string
		sat   bool
	}{{"-1", false}, {"0", true}, {"2", true}} {
		t.Run(tc.value, func(t *testing.T) {
			value := f.own(f.r.manager.ArithmeticRealNumeralText(tc.value))
			equality := f.own(f.r.manager.MkEq(k, value))
			checkGoal := f.goal(false, false)
			checkResult := f.result()
			fmMust(t, checkGoal.Assert(projected))
			fmMust(t, checkGoal.Assert(equality))
			check, cause := NewNLQSATTactic(f.r.manager, f.r.config)
			fmMust(t, cause)
			t.Cleanup(func() { fmMust(t, check.Close()) })
			fmMust(t, check.Apply(checkGoal, checkResult))
			if tc.sat {
				if size, cause := checkGoal.Size(); cause != nil || size != 0 {
					t.Fatal("projected formula rejected witness", size, cause)
				}
			} else if !checkGoal.IsDecidedUnsat() {
				t.Fatal("projected formula admitted impossible value")
			}
		})
	}
}

func TestNLQETacticEliminatesUniversalAndAlternatingNonlinear(t *testing.T) {
	for _, tc := range []struct {
		name   string
		build  func(*fmFixture, ast.ASTHandle) ast.ASTHandle
		values map[string]bool
	}{
		{name: "universal", values: map[string]bool{"-1": true, "0": true, "1": false}, build: func(f *fmFixture, k ast.ASTHandle) ast.ASTHandle {
			x := f.variable(0, f.real)
			square := f.own(f.r.manager.MkArithmeticMul(x, x))
			body := f.own(f.r.manager.MkArithmeticGE(square, k))
			name, err := f.r.manager.Symbols.Intern("nlqe-universal-x")
			fmMust(t, err)
			return f.own(f.r.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.real}, Names: []usymbol.Symbol{name}, Body: body}))
		}},
		{name: "alternating", values: map[string]bool{"-1": false, "0": true, "1": true}, build: func(f *fmFixture, k ast.ASTHandle) ast.ASTHandle {
			y, x := f.variable(0, f.real), f.variable(1, f.real)
			xSquare := f.own(f.r.manager.MkArithmeticMul(x, x))
			ySquare := f.own(f.r.manager.MkArithmeticMul(y, y))
			right := f.own(f.r.manager.MkArithmeticAdd(ySquare, k))
			body := f.own(f.r.manager.MkArithmeticLE(xSquare, right))
			yName, err := f.r.manager.Symbols.Intern("nlqe-alternating-y")
			fmMust(t, err)
			xName, err := f.r.manager.Symbols.Intern("nlqe-alternating-x")
			fmMust(t, err)
			inner := f.own(f.r.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.real}, Names: []usymbol.Symbol{yName}, Body: body}))
			return f.own(f.r.manager.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.real}, Names: []usymbol.Symbol{xName}, Body: inner}))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFMFixture(t)
			k := f.constant("nlqe-parameter", f.real)
			eliminate, err := NewNLQETactic(f.r.manager, f.r.config)
			fmMust(t, err)
			t.Cleanup(func() { fmMust(t, eliminate.Close()) })
			goal := f.goal(false, false)
			result := f.result()
			fmMust(t, goal.Assert(tc.build(f, k)))
			fmMust(t, eliminate.Apply(goal, result))
			projected, err := goal.Form(0)
			fmMust(t, err)
			if quantified, cause := f.r.manager.HasQuantifiers(projected); cause != nil || quantified {
				t.Fatal("nonlinear QE retained a quantifier", quantified, cause)
			}
			for valueText, expected := range tc.values {
				valueText, expected := valueText, expected
				t.Run(valueText, func(t *testing.T) {
					value := f.own(f.r.manager.ArithmeticRealNumeralText(valueText))
					equality := f.own(f.r.manager.MkEq(k, value))
					checkGoal := f.goal(false, false)
					checkResult := f.result()
					fmMust(t, checkGoal.Assert(projected))
					fmMust(t, checkGoal.Assert(equality))
					check, cause := NewNLQSATTactic(f.r.manager, f.r.config)
					fmMust(t, cause)
					t.Cleanup(func() { fmMust(t, check.Close()) })
					fmMust(t, check.Apply(checkGoal, checkResult))
					if expected {
						if size, cause := checkGoal.Size(); cause != nil || size != 0 {
							t.Fatal("projected formula rejected witness", size, cause)
						}
					} else if !checkGoal.IsDecidedUnsat() {
						t.Fatal("projected formula admitted impossible value")
					}
				})
			}
		})
	}
}

func TestNLQETacticComposesWithNLQSATForFreeParameters(t *testing.T) {
	f := newFMFixture(t)
	k := f.constant("nlqe-compose-k", f.real)
	x := f.variable(0, f.real)
	square := f.own(f.r.manager.MkArithmeticMul(x, x))
	body := f.own(f.r.manager.MkEq(square, k))
	name, err := f.r.manager.Symbols.Intern("nlqe-compose-x")
	fmMust(t, err)
	quantified := f.own(f.r.manager.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.real}, Names: []usymbol.Symbol{name}, Body: body}))
	two := f.own(f.r.manager.ArithmeticRealNumeralText("2"))
	equality := f.own(f.r.manager.MkEq(k, two))
	eliminate, err := NewNLQETactic(f.r.manager, f.r.config)
	fmMust(t, err)
	check, err := NewNLQSATTactic(f.r.manager, f.r.config)
	fmMust(t, err)
	combined, err := basetactic.NewAndThenTactic(eliminate, check)
	fmMust(t, err)
	fmMust(t, combined.IncRef())
	t.Cleanup(func() { fmMust(t, combined.DecRef()) })
	goal := f.goal(false, false)
	result := f.result()
	fmMust(t, goal.Assert(equality))
	fmMust(t, goal.Assert(quantified))
	fmMust(t, combined.Apply(goal, result))
	if !goal.IsDecidedSat() {
		t.Fatal("composed nonlinear elimination did not decide the projected goal")
	}
}

func TestNLQAnswerSimplificationUsesPriorProjectedClauses(t *testing.T) {
	f := newFMFixture(t)
	simplifier, err := astrewriter.NewExprSafeReplace(f.r.manager)
	fmMust(t, err)
	search := &nlqSearch{manager: f.r.manager, answerSimplify: simplifier}
	t.Cleanup(func() {
		for i := len(search.answers) - 1; i >= 0; i-- {
			fmMust(t, f.r.manager.DelRef(&search.answers[i]))
		}
		fmMust(t, simplifier.Close())
	})
	p := f.constant("nlqe-answer-p", f.boolean)
	notP := f.own(f.r.manager.MkNot(p))
	first, err := f.r.manager.NewRef(notP)
	fmMust(t, err)
	fmMust(t, search.addAnswer(&first))
	second, err := f.r.manager.NewRef(p)
	fmMust(t, err)
	fmMust(t, search.addAnswer(&second))
	rewritten, err := f.r.manager.RefGet(search.answers[1])
	fmMust(t, err)
	if !f.r.manager.IsFalse(rewritten) {
		t.Fatal("prior negative projection did not simplify its atom to false")
	}
}
