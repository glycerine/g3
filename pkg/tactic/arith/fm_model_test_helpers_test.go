package arith

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"testing"
)

func fmModelFixture(t *testing.T) (*FMModelConverter, *ast.ASTManager) {
	t.Helper()
	base, m := fixDLFixture(t, false)
	c, e := NewFMModelConverter(m, base.services)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, c.Close()) })
	return c, m
}
func fmApply(t *testing.T, c *FMModelConverter, md *model.Model) *model.ModelCore {
	t.Helper()
	var ref tactic.ModelRef
	equationMust(t, ref.Set(&md.CoreField))
	t.Cleanup(func() { equationMust(t, ref.Close()) })
	equationMust(t, c.ApplyModel(&ref, 0))
	got, e := ref.Get()
	equationMust(t, e)
	return got
}
func fmValue(t *testing.T, m *ast.ASTManager, md *model.ModelCore, x ast.ASTHandle) string {
	t.Helper()
	d, e := m.GetDecl(x)
	equationMust(t, e)
	h, e := md.ConstantInterpretation(d)
	equationMust(t, e)
	s, _, yes, e := m.ArithmeticNumeralText(h)
	equationMust(t, e)
	if !yes {
		t.Fatal("not numeral")
	}
	return s
}
