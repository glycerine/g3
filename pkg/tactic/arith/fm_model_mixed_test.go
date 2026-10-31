package arith

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestFMModelToRealEliminatedIntegerBounds(t *testing.T) {
	for _, lower := range []bool{false, true} {
		for _, strict := range []bool{false, true} {
			t.Run(fmt.Sprintf("lower%v/strict%v", lower, strict), func(t *testing.T) {
				c, m := fmModelFixture(t)
				x := fixDLConst(t, m, true, "x")
				y := fixDLConst(t, m, false, "y")
				realX := fixDLApp(t, m, 14, x)
				product := fixDLApp(t, m, 9, fixDLNumber(t, m, false, "2"), realX)
				sum := fixDLApp(t, m, 6, product, y)
				op := int32(2)
				if lower {
					op = 3
				}
				if strict {
					op = 5 - op
				}
				bound := fixDLApp(t, m, op, sum, fixDLNumber(t, m, false, "7/2"))
				if strict {
					neg, err := m.MkNot(bound)
					equationMust(t, err)
					bound = neg
					fixDLPin(t, m, bound)
				}
				xd, err := m.GetDecl(x)
				equationMust(t, err)
				equationMust(t, c.Insert(xd, []ast.ASTHandle{bound}))
				md, err := model.NewModel(m)
				equationMust(t, err)
				t.Cleanup(func() { equationMust(t, md.Close()) })
				yd, err := m.GetDecl(y)
				equationMust(t, err)
				equationMust(t, md.Core().RegisterConstant(yd, fixDLNumber(t, m, false, "1/2")))
				core := fmApply(t, c, md)
				want := "1"
				if lower {
					want = "2"
				}
				if got := fmValue(t, m, core, x); got != want {
					t.Fatal("integer bound rounding", got, want)
				}
			})
		}
	}
}
func TestFMTacticOneSidedMixedIntegerModel(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	name := "fm_real_only"
	equationMust(t, fm.context.SetBoolName(&fm.params, &name, false))
	equationMust(t, fm.UpdateParams(&fm.params))
	x := fixDLConst(t, m, true, "x")
	y := fixDLConst(t, m, false, "y")
	sum := fixDLApp(t, m, 6, fixDLApp(t, m, 14, x), y)
	bound := fixDLApp(t, m, 2, sum, fixDLNumber(t, m, false, "7/2"))
	g := fixDLGoal(t, m, false, true, false, fmPredicate(t, m, y), bound)
	r := fmResult(t)
	equationMust(t, fm.Apply(g, r))
	size, err := r.Goals[0].Size()
	equationMust(t, err)
	if size != 1 || fm.run.eliminated != 1 || r.ModelConverter == nil {
		t.Fatal("integer zero-cost elimination", size, fm.run.eliminated)
	}
	md, err := model.NewModel(m)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, md.Close()) })
	yd, err := m.GetDecl(y)
	equationMust(t, err)
	equationMust(t, md.Core().RegisterConstant(yd, fixDLNumber(t, m, false, "1/2")))
	core := fmApply(t, r.ModelConverter.(*FMModelConverter), md)
	if got := fmValue(t, m, core, x); got != "3" {
		t.Fatal("mixed integer reconstruction", got)
	}
}
