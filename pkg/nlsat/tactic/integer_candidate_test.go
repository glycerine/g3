package tactic

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	basetactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func candidateGoal(t *testing.T, f *goalCompilerFixture, models bool) (*basetactic.Goal, []ast.ASTHandle) {
	t.Helper()
	m := f.manager
	goal, err := basetactic.NewGoal(m, false, models, true)
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, goal.Close()) })
	two, err := m.ArithmeticRealNumeralText("2")
	compilerMust(t, err)
	sort, err := m.GetSort(two)
	compilerMust(t, err)
	xs := make([]ast.ASTHandle, minCandidateAssertions)
	for i := range xs {
		xs[i] = compilerConst(t, m, fmt.Sprintf("x%d", i), sort)
		square, err := m.MkArithmeticMul(xs[i], xs[i])
		compilerMust(t, err)
		bound, err := m.MkArithmeticGT(square, two)
		compilerMust(t, err)
		compilerMust(t, goal.Assert(bound))
	}
	return goal, xs
}

func candidateTactic(t *testing.T, f *goalCompilerFixture) *NLSATTactic {
	t.Helper()
	tac, err := NewIntegerModelTactic(f.manager, f.context, f.globals, nil, f.services)
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, tac.Close()) })
	return tac
}

func TestIntegerCandidateCertifiesOriginalAssertions(t *testing.T) {
	if ast.ASTSourceDebug || diagnostic.TraceCompiled() {
		t.Skip("this model search runs with tracing and source assertions disabled")
	}
	for _, tc := range []struct{ models, compound bool }{{true, false}, {false, false}, {true, true}, {false, true}} {
		t.Run(fmt.Sprintf("model=%v/compound=%v", tc.models, tc.compound), func(t *testing.T) {
			f := newGoalCompilerFixture(t)
			m := f.manager
			goal, xs := candidateGoal(t, f, tc.models)
			four, err := m.ArithmeticRealNumeralText("4")
			compilerMust(t, err)
			negativeFour, err := m.MkArithmeticNeg(four)
			compilerMust(t, err)
			upper, err := m.MkArithmeticLE(xs[0], negativeFour)
			compilerMust(t, err)
			compilerMust(t, goal.Assert(upper))
			zero, err := m.ArithmeticIntegerNumeralText("0")
			compilerMust(t, err)
			intSort, err := m.GetSort(zero)
			compilerMust(t, err)
			z := compilerConst(t, m, "z", intSort)
			family, err := m.ArithmeticFamilyID()
			compilerMust(t, err)
			coerced, err := m.MkFamilyApp(family, candidateToReal, nil, []ast.ASTHandle{z})
			compilerMust(t, err)
			fixed, err := m.MkEq(coerced, negativeFour)
			compilerMust(t, err)
			compilerMust(t, goal.Assert(fixed))
			for i := 1; i < len(xs); i++ {
				bound, err := m.MkArithmeticGT(xs[i], xs[i-1])
				compilerMust(t, err)
				compilerMust(t, goal.Assert(bound))
			}
			if tc.compound {
				product, err := m.MkArithmeticMul(xs[1], xs[2])
				compilerMust(t, err)
				two, err := m.ArithmeticRealNumeralText("2")
				compilerMust(t, err)
				bound, err := m.MkArithmeticGT(product, two)
				compilerMust(t, err)
				compilerMust(t, goal.Assert(bound))
			}
			original, err := basetactic.NewGoalCopy(goal)
			compilerMust(t, err)
			t.Cleanup(func() { compilerMust(t, original.Close()) })
			var result basetactic.TacticResult
			t.Cleanup(func() { compilerMust(t, result.Close()) })
			compilerMust(t, candidateTactic(t, f).Apply(goal, &result))
			if !result.DecidedSat() || (result.ModelConverter != nil) != tc.models {
				t.Fatalf("decided SAT %v, model converter %T", result.DecidedSat(), result.ModelConverter)
			}
			if !tc.models {
				return
			}
			var returned basetactic.ModelRef
			compilerMust(t, result.ModelConverter.ApplyUnindexed(&returned))
			t.Cleanup(func() { compilerMust(t, returned.Close()) })
			core, err := returned.Get()
			compilerMust(t, err)
			ev, err := model.NewModelEvaluator(core, f.services)
			compilerMust(t, err)
			t.Cleanup(func() { compilerMust(t, ev.Close()) })
			compilerMust(t, ev.SetModelCompletion(false))
			out, err := m.NewRef(ast.ASTHandle{})
			compilerMust(t, err)
			t.Cleanup(func() { compilerMust(t, m.DelRef(&out)) })
			forms, err := original.AppendFormulas(nil)
			compilerMust(t, err)
			for i, h := range forms {
				compilerMust(t, ev.Evaluate(h, out))
				value, err := m.RefGet(out)
				compilerMust(t, err)
				if !m.IsTrue(value) {
					t.Fatalf("published model falsifies original assertion %d", i)
				}
			}
		})
	}
}

func TestIntegerCandidateMissPreservesGoal(t *testing.T) {
	for _, which := range []string{"contradiction", "compound-false", "rational", "non-integral-root", "cycle", "large-DAG", "resource-limit", "canceled", "solver-memory", "solver-conflicts", "imprecise", "proof-manager", "translated-miss"} {
		t.Run(which, func(t *testing.T) {
			f := newGoalCompilerFixture(t)
			m := f.manager
			goal, xs := candidateGoal(t, f, true)
			tac := candidateTactic(t, f)
			zero, err := m.ArithmeticRealNumeralText("0")
			compilerMust(t, err)
			var formula ast.ASTHandle
			switch which {
			case "contradiction", "translated-miss":
				formula, err = m.MkEq(xs[0], zero)
				if which == "translated-miss" {
					copyTactic, e := tac.Translate(m)
					compilerMust(t, e)
					tac = copyTactic.(*NLSATTactic)
					t.Cleanup(func() { compilerMust(t, tac.Close()) })
					if !tac.modelOnly {
						t.Fatal("translation changed tactic mode")
					}
				}
			case "compound-false":
				product, e := m.MkArithmeticMul(xs[0], xs[1])
				compilerMust(t, e)
				formula, err = m.MkArithmeticLT(product, zero)
			case "rational":
				fraction, e := m.ArithmeticRealNumeralText("7/3")
				compilerMust(t, e)
				formula, err = m.MkEq(xs[0], fraction)
			case "non-integral-root":
				three, e := m.ArithmeticRealNumeralText("3")
				compilerMust(t, e)
				square, e := m.MkArithmeticMul(xs[0], xs[0])
				compilerMust(t, e)
				formula, err = m.MkEq(square, three)
			case "cycle":
				first, e := m.MkArithmeticLE(xs[0], xs[1])
				compilerMust(t, e)
				compilerMust(t, goal.Assert(first))
				formula, err = m.MkArithmeticLE(xs[1], xs[0])
			case "large-DAG":
				h := xs[0]
				for range 18 {
					h, err = m.MkArithmeticMul(h, h)
					compilerMust(t, err)
				}
				formula, err = m.MkArithmeticGT(h, zero)
			case "resource-limit":
				compilerMust(t, m.Limit.Push(12))
				defer m.Limit.Pop()
			case "canceled":
				m.Limit.Cancel()
				defer m.Limit.DecCancel()
			case "solver-memory", "solver-conflicts":
				key := "nlsat.max_conflicts"
				if which == "solver-memory" {
					key = "nlsat.max_memory"
				}
				compilerMust(t, f.globals.SourceSetName(key, "1"))
				t.Cleanup(func() { compilerMust(t, f.globals.SourceReset()) })
			case "imprecise":
				compilerMust(t, goal.SetPrecision(basetactic.GoalOver))
			case "proof-manager":
				compilerMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
				defer func() { compilerMust(t, m.ToggleProofMode(ast.ASTProofDisabled)) }()
			}
			compilerMust(t, err)
			if !formula.IsNull() {
				compilerMust(t, goal.Assert(formula))
			}
			before, err := goal.AppendFormulas(nil)
			compilerMust(t, err)
			compilerMust(t, goal.SetDepth(7))
			var result basetactic.TacticResult
			t.Cleanup(func() { compilerMust(t, result.Close()) })
			compilerMust(t, tac.Apply(goal, &result))
			after, err := goal.AppendFormulas(nil)
			compilerMust(t, err)
			depth, err := goal.Depth()
			compilerMust(t, err)
			if len(before) != len(after) || depth != 7 || len(result.Goals) != 1 || result.Goals[0] != goal || result.ModelConverter != nil || !result.Core.IsNull() {
				t.Fatalf("miss changed goal (depth %d, forms %d to %d, model %T)", depth, len(before), len(after), result.ModelConverter)
			}
			for i, h := range before {
				if h != after[i] {
					t.Fatalf("miss replaced original assertion %d", i)
				}
			}
		})
	}
}

func TestIntegerCandidateCheckedSquare(t *testing.T) {
	square := candidateTerm{kind: candidateSquare}
	for _, x := range []int64{-maxInt64SquareRoot - 1, -maxInt64SquareRoot, -3, 0, maxInt64SquareRoot, maxInt64SquareRoot + 1} {
		vars := []candidateVariable{{numeral: candidateNumeral{value: x}}}
		value, ok := square.value(vars)
		inRange := x >= -maxInt64SquareRoot && x <= maxInt64SquareRoot
		if ok != inRange || ok && value != x*x {
			t.Fatalf("square(%d) = %d, %v", x, value, ok)
		}
	}
}
