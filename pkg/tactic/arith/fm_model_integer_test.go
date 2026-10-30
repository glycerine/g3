package arith

// Source: src/tactic/arith/fm_tactic.cpp:125-151,180-189.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"testing"
)

func TestFMModelConverterIntegerStrictRounding(t *testing.T) {
	c, m := fmModelFixture(t)
	x := fixDLConst(t, m, true, "fm-int-x")
	three := fixDLNumber(t, m, true, "3")
	le, e := m.MkArithmeticLE(x, three)
	equationMust(t, e)
	strict, e := m.MkNot(le)
	equationMust(t, e)
	d, e := m.GetDecl(x)
	equationMust(t, e)
	equationMust(t, c.Insert(d, []ast.ASTHandle{strict}))
	md, e := model.NewModel(m)
	equationMust(t, e)
	got := fmApply(t, c, md)
	if value := fmValue(t, m, got, x); value != "4" {
		t.Fatal("strict integer lower", value)
	}
}
