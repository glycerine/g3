package qe_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArithmeticBranchDefinitionStrictReal(t *testing.T) {
	f := newArithmeticPluginFixture(t, true)
	z := f.realNumeral(t, "0")
	m1 := f.realNumeral(t, "-1")
	lo := f.pin(f.manager.MkArithmeticLE(f.pin(f.manager.MkArithmeticMul(m1, f.rx)), z))
	up := f.pin(f.manager.MkArithmeticLT(f.pin(f.manager.MkArithmeticAdd(f.rx, m1)), z))
	form := f.pin(f.manager.MkAnd(lo, up))
	ok, e := f.plugin.Prepare(f.contains, form, []ast.ASTHandle{lo, up}, nil)
	pluginMust(t, e)
	if !ok {
		t.Fatal("prepare")
	}
	s := f.manager.NumeralService()
	v, e := s.NewValue()
	pluginMust(t, e)
	defer s.Del(&v)
	pluginMust(t, s.SetUnsigned(&v, 1))
	pluginMust(t, f.plugin.Assign(f.contains, form, &v))
}
