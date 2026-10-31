package arith

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestPurifyToIntFloorConstraintsAndCachedProof(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprintf("proofs=%v", proofs), func(t *testing.T) {
			runner, m := qfnraPurifier(t, proofs, true)
			x := fixDLConst(t, m, false, "to-int-x")
			term := fixDLApp(t, m, qfnraPurifyToInt, x)
			zero := fixDLNumber(t, m, true, "0")
			formula, err := m.MkEq(term, zero)
			equationMust(t, err)
			fixDLPin(t, m, formula)
			goal, err := tactic.NewGoal(m, proofs, true, true)
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, goal.Close()) })
			var proof ast.ASTHandle
			if proofs {
				proof, err = m.MkProofAsserted(formula)
				equationMust(t, err)
			}
			dependency, err := m.DependencyLeaf(formula)
			equationMust(t, err)
			for j := 0; j < 2; j++ {
				equationMust(t, goal.AssertWithProof(formula, proof, dependency))
			}
			var result tactic.TacticResult
			t.Cleanup(func() { equationMust(t, result.Close()) })
			equationMust(t, runner.Apply(goal, &result))
			size, err := goal.Size()
			equationMust(t, err)
			if size != 4 || result.ModelConverter == nil {
				t.Fatalf("got %d formulas and model converter %v", size, result.ModelConverter)
			}
			first, err := goal.Form(0)
			equationMust(t, err)
			second, err := goal.Form(1)
			equationMust(t, err)
			fresh, err := m.GetArg(first, 0)
			equationMust(t, err)
			again, err := m.GetArg(second, 0)
			equationMust(t, err)
			if fresh != again || fresh == term {
				t.Fatal("repeated to_int did not reuse its fresh integer")
			}
			for j := uint32(0); j < size; j++ {
				if proofs {
					p, e := goal.Proof(j)
					equationMust(t, e)
					fact, e := m.GetProofFact(p)
					equationMust(t, e)
					form, e := goal.Form(j)
					equationMust(t, e)
					if fact != form {
						t.Fatalf("proof %d has wrong conclusion", j)
					}
				}
			}
			xDecl, err := m.GetDecl(x)
			equationMust(t, err)
			freshDecl, err := m.GetDecl(fresh)
			equationMust(t, err)
			for _, row := range []struct {
				x     string
				floor int
			}{{"-3/2", -2}, {"-1", -1}, {"0", 0}, {"3/2", 1}} {
				for k := -3; k <= 2; k++ {
					md, e := model.NewModel(m)
					equationMust(t, e)
					xValue := fixDLNumber(t, m, false, row.x)
					kValue := fixDLNumber(t, m, true, fmt.Sprint(k))
					equationMust(t, md.CoreField.RegisterConstant(xDecl, xValue))
					equationMust(t, md.CoreField.RegisterConstant(freshDecl, kValue))
					ev, e := model.NewModelEvaluator(&md.CoreField, runner.services)
					equationMust(t, e)
					out, e := m.NewRef(ast.ASTHandle{})
					equationMust(t, e)
					valid := true
					for j := uint32(2); j < 4; j++ {
						form, e := goal.Form(j)
						equationMust(t, e)
						equationMust(t, ev.Evaluate(form, out))
						v, e := m.RefGet(out)
						equationMust(t, e)
						if !m.IsTrue(v) && !m.IsFalse(v) {
							t.Fatalf("constraint did not evaluate: %v", v)
						}
						valid = valid && m.IsTrue(v)
					}
					equationMust(t, m.DelRef(&out))
					equationMust(t, ev.Close())
					equationMust(t, md.Close())
					if valid != (k == row.floor) {
						t.Fatalf("x=%s k=%d valid=%v", row.x, k, valid)
					}
				}
			}
		})
	}
}
