package arith

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestEq2BVApplyRewritesGoalAndRestoresModel(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			base, m := fixDLFixture(t, proofs)
			bounds, e := NewBoundManager(m, base.context, base.globals)
			equationMust(t, e)
			t.Cleanup(func() { equationMust(t, bounds.Close()) })
			x := fixDLConst(t, m, true, "apply-x")
			num := func(s string) ast.ASTHandle { return fixDLNumber(t, m, true, s) }
			lower := fixDLApp(t, m, 3, x, num("0"))
			upper := fixDLApp(t, m, 2, x, num("7"))
			x1 := collectorEq(t, m, x, num("1"))
			x3 := collectorEq(t, m, x, num("3"))
			disj, e := m.MkOrArgs([]ast.ASTHandle{x1, x3})
			equationMust(t, e)
			fixDLPin(t, m, disj)
			goal := fixDLGoal(t, m, proofs, true, false, lower, upper, disj)
			var result tactic.TacticResult
			t.Cleanup(func() { equationMust(t, result.Close()) })
			equationMust(t, eq2bvApply(goal, bounds, &result))
			depth, e := goal.Depth()
			equationMust(t, e)
			if depth != 1 || result.ModelConverter == nil {
				t.Fatal("missing goal transformation")
			}
			form, e := goal.Form(2)
			equationMust(t, e)
			equality, e := m.GetArg(form, 0)
			equationMust(t, e)
			fresh, e := m.GetArg(equality, 0)
			equationMust(t, e)
			sort, e := m.GetSort(fresh)
			equationMust(t, e)
			width, e := m.BitVectorSize(sort)
			equationMust(t, e)
			if width != 2 {
				t.Fatalf("width %d", width)
			}
			size, e := goal.Size()
			equationMust(t, e)
			if size != 5 {
				t.Fatalf("goal size %d", size)
			}
			for i := uint32(0); i < size; i++ {
				f, e := goal.Form(i)
				equationMust(t, e)
				has, e := m.OccursExpr(x, f)
				equationMust(t, e)
				if has {
					t.Fatal("integer variable survived conversion")
				}
				if proofs && i >= 2 {
					pr, e := goal.Proof(i)
					equationMust(t, e)
					fact, e := m.GetProofFact(pr)
					equationMust(t, e)
					if fact != f {
						t.Fatal("wrong proof conclusion")
					}
				}
			}
			md, e := model.NewModel(m)
			equationMust(t, e)
			freshDecl, e := m.GetDecl(fresh)
			equationMust(t, e)
			three, e := m.BitVectorNumeralUnsigned(3, width)
			equationMust(t, e)
			equationMust(t, md.Core().RegisterConstant(freshDecl, three))
			var ref tactic.ModelRef
			equationMust(t, ref.Set(md.Core()))
			defer func() { equationMust(t, ref.Close()) }()
			equationMust(t, result.ModelConverter.ApplyUnindexed(&ref))
			converted, e := ref.Get()
			equationMust(t, e)
			originalDecl, e := m.GetDecl(x)
			equationMust(t, e)
			value, e := converted.ConstantInterpretation(originalDecl)
			equationMust(t, e)
			text, _, found, e := m.ArithmeticNumeralText(value)
			equationMust(t, e)
			if !found || text != "3" {
				t.Fatalf("restored model %q", text)
			}
		})
	}
}
func TestEq2BVApplyLeavesGoalWithoutCandidates(t *testing.T) {
	bounds, m := boundManagerFixture(t)
	x := fixDLConst(t, m, true, "unchanged-x")
	y := fixDLConst(t, m, true, "unchanged-y")
	form := collectorEq(t, m, x, y)
	goal := fixDLGoal(t, m, false, false, false, form)
	var result tactic.TacticResult
	defer func() { equationMust(t, result.Close()) }()
	equationMust(t, eq2bvApply(goal, bounds, &result))
	depth, e := goal.Depth()
	equationMust(t, e)
	got, e := goal.Form(0)
	equationMust(t, e)
	if depth != 0 || got != form || result.ModelConverter != nil {
		t.Fatal("non-candidate goal changed")
	}
}

func TestEq2BVApplyPreservesBoundsOfNonCandidates(t *testing.T) {
	bounds, m := boundManagerFixture(t)
	x := fixDLConst(t, m, true, "candidate-x")
	y := fixDLConst(t, m, true, "noncandidate-y")
	one := fixDLNumber(t, m, true, "1")
	zero := fixDLNumber(t, m, true, "0")
	yBound := fixDLApp(t, m, 3, y, zero)
	sum := fixDLApp(t, m, 6, y, one)
	yUse := collectorEq(t, m, sum, one)
	xUse := collectorEq(t, m, x, one)
	goal := fixDLGoal(t, m, false, false, false, yBound, yUse, xUse)
	var result tactic.TacticResult
	defer func() { equationMust(t, result.Close()) }()
	equationMust(t, eq2bvApply(goal, bounds, &result))
	kept, e := goal.Form(0)
	equationMust(t, e)
	if kept != yBound {
		t.Fatal("non-candidate bound was removed")
	}
}
