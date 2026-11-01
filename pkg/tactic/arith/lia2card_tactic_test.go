package arith

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestLIA2CardGoalPreservesBoundedArithmeticSemantics(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, compiled := range []bool{false, true} {
			t.Run(fmt.Sprintf("proofs=%v/equality=%v", proofs, compiled), func(t *testing.T) {
				base, m := fixDLFixture(t, proofs)
				equationMust(t, m.RegisterDeclarationPlugins())
				params, err := base.context.NewValue()
				equationMust(t, err)
				defer func() { equationMust(t, base.context.Del(&params)) }()
				key := "compile_equality"
				equationMust(t, base.context.SetBoolName(&params, &key, compiled))
				port, err := NewLIA2CardTactic(m, base.context, base.globals, &params)
				equationMust(t, err)
				defer func() { equationMust(t, port.Close()) }()
				x := fixDLConst(t, m, true, "lia2card-goal-x")
				y := fixDLConst(t, m, true, "lia2card-goal-y")
				zero := fixDLNumber(t, m, true, "0")
				one := fixDLNumber(t, m, true, "1")
				lowerX := fixDLApp(t, m, 3, x, zero)
				upperX := fixDLApp(t, m, 2, x, one)
				lowerY := fixDLApp(t, m, 3, y, zero)
				upperY := fixDLApp(t, m, 2, y, one)
				sum := fixDLApp(t, m, 6, x, y)
				eq := collectorEq(t, m, sum, one)
				goal := fixDLGoal(t, m, proofs, false, false, lowerX, upperX, lowerY, upperY, eq)
				var result tactic.TacticResult
				defer func() { equationMust(t, result.Close()) }()
				equationMust(t, port.Apply(goal, &result))
				if len(result.Goals) != 1 || result.Goals[0] != goal {
					t.Fatal("goal not published")
				}
				count, err := goal.Size()
				equationMust(t, err)
				if count < 7 {
					t.Fatal("missing 0/1 bounds", count)
				}
				rewritten, err := goal.Form(4)
				equationMust(t, err)
				if rewritten == eq {
					t.Fatal("arithmetic equality was not converted")
				}
				if compiled {
					isPB, e := m.IsPseudoBooleanEQ(rewritten)
					equationMust(t, e)
					if !isPB {
						t.Fatal("compile_equality did not select pseudo-Boolean equality")
					}
				}
				depth, err := goal.Depth()
				equationMust(t, err)
				if depth != 1 {
					t.Fatal("goal depth", depth)
				}
				if proofs {
					for i := uint32(0); i < count; i++ {
						form, e := goal.Form(i)
						equationMust(t, e)
						proof, e := goal.Proof(i)
						equationMust(t, e)
						fact, e := m.GetProofFact(proof)
						equationMust(t, e)
						if fact != form {
							t.Fatal("proof conclusion", i)
						}
					}
				}
				for xv := -1; xv <= 2; xv++ {
					for yv := -1; yv <= 2; yv++ {
						md, e := model.NewModel(m)
						equationMust(t, e)
						for variable, integer := range map[ast.ASTHandle]int{x: xv, y: yv} {
							decl, e := m.GetDecl(variable)
							equationMust(t, e)
							value, e := m.ArithmeticIntegerNumeralText(fmt.Sprint(integer))
							equationMust(t, e)
							equationMust(t, md.Core().RegisterConstant(decl, value))
						}
						eval, e := model.NewModelEvaluator(md.Core(), base.services)
						equationMust(t, e)
						actual := true
						for i := uint32(0); i < count; i++ {
							form, e := goal.Form(i)
							equationMust(t, e)
							ref, e := eval.EvaluateValue(form)
							equationMust(t, e)
							value, e := m.RefGet(ref)
							equationMust(t, e)
							if !m.IsTrue(value) && !m.IsFalse(value) {
								t.Fatal("non-ground result", value)
							}
							actual = actual && m.IsTrue(value)
							equationMust(t, m.DelRef(&ref))
						}
						want := xv >= 0 && xv <= 1 && yv >= 0 && yv <= 1 && xv+yv == 1
						if actual != want {
							t.Fatalf("x=%d y=%d got %v want %v", xv, yv, actual, want)
						}
						equationMust(t, eval.Close())
						equationMust(t, md.Close())
					}
				}
			})
		}
	}
}

func TestLIA2CardContextTranslationCopiesParameterSnapshot(t *testing.T) {
	source, m := fixDLFixture(t, false)
	params, err := source.context.NewValue()
	equationMust(t, err)
	key := "compile_equality"
	equationMust(t, source.context.SetBoolName(&params, &key, true))
	strategy, err := NewLIA2CardTactic(m, source.context, source.globals, &params)
	equationMust(t, err)
	equationMust(t, source.context.SetBoolName(&params, &key, false))
	equationMust(t, source.context.Del(&params))
	target, targetManager := fixDLFixture(t, false)
	translated, matched, err := TranslateContextLeaf(strategy, targetManager, target.context, target.globals, target.services)
	equationMust(t, err)
	if !matched {
		t.Fatal("lia2card translation was not dispatched")
	}
	copy := translated.(*LIA2CardTactic)
	if !copy.compileEquality || copy.context != target.context {
		t.Fatal("translation lost parameter snapshot or target services")
	}
	equationMust(t, strategy.Close())
	x := fixDLConst(t, targetManager, true, "translated-card-x")
	zero := fixDLNumber(t, targetManager, true, "0")
	one := fixDLNumber(t, targetManager, true, "1")
	lower := fixDLApp(t, targetManager, 3, x, zero)
	upper := fixDLApp(t, targetManager, 2, x, one)
	goal := fixDLGoal(t, targetManager, false, false, false, lower, upper)
	var result tactic.TacticResult
	equationMust(t, copy.Apply(goal, &result))
	equationMust(t, result.Close())
	equationMust(t, copy.Close())
}
