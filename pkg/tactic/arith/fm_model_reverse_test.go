package arith

// Source: src/tactic/arith/fm_tactic.cpp:161-163, reverse eliminated-variable reconstruction.
import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestFMModelConverterReconstructsVariablesInReverse(t *testing.T) {
	c, m := fmModelFixture(t)
	x := fixDLConst(t, m, false, "fm-reverse-x")
	y := fixDLConst(t, m, false, "fm-reverse-y")
	zero := fixDLNumber(t, m, false, "0")
	minusOne := fixDLNumber(t, m, false, "-1")
	four := fixDLNumber(t, m, false, "4")
	negativeY, e := m.MkArithmeticMul(minusOne, y)
	equationMust(t, e)
	left, e := m.MkArithmeticAdd(x, negativeY)
	equationMust(t, e)
	xUpper, e := m.MkArithmeticLE(left, zero)
	equationMust(t, e)
	yLower, e := m.MkArithmeticGE(y, four)
	equationMust(t, e)
	xd, e := m.GetDecl(x)
	equationMust(t, e)
	yd, e := m.GetDecl(y)
	equationMust(t, e)
	equationMust(t, c.Insert(xd, []ast.ASTHandle{xUpper}))
	equationMust(t, c.Insert(yd, []ast.ASTHandle{yLower}))
	md, e := model.NewModel(m)
	equationMust(t, e)
	got := fmApply(t, c, md)
	if xv, yv := fmValue(t, m, got, x), fmValue(t, m, got, y); xv != "4" || yv != "5" {
		t.Fatal("reverse reconstruction", xv, yv)
	}
}
