package arith

// Source: src/tactic/arith/fm_tactic.cpp:57-194.
import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestFMModelConverterRealBoundsAndMidpoint(t *testing.T) {
	c, m := fmModelFixture(t)
	x := fixDLConst(t, m, false, "fm-x")
	y := fixDLConst(t, m, false, "fm-y")
	two := fixDLNumber(t, m, false, "2")
	ten := fixDLNumber(t, m, false, "10")
	four := fixDLNumber(t, m, false, "4")
	twox, e := m.MkArithmeticMul(two, x)
	equationMust(t, e)
	lhs, e := m.MkArithmeticAdd(twox, y)
	equationMust(t, e)
	upper, e := m.MkArithmeticLE(lhs, ten)
	equationMust(t, e)
	lower, e := m.MkArithmeticGE(x, two)
	equationMust(t, e)
	xd, e := m.GetDecl(x)
	equationMust(t, e)
	equationMust(t, c.Insert(xd, []ast.ASTHandle{upper, lower}))
	md, e := model.NewModel(m)
	equationMust(t, e)
	yd, e := m.GetDecl(y)
	equationMust(t, e)
	equationMust(t, md.CoreField.RegisterConstant(yd, four))
	got := fmApply(t, c, md)
	if value := fmValue(t, m, got, x); value != "5/2" {
		t.Fatal("midpoint", value)
	}
}
