package qe_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArithmeticBranchDefinitionTransfer(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	z := f.integerNumeral(t, "0")
	a := f.pin(f.manager.MkArithmeticLE(f.x, z))
	neg := f.integerNumeral(t, "-1")
	l0 := f.pin(f.manager.MkArithmeticLE(f.pin(f.manager.MkArithmeticMul(neg, f.x)), z))
	l1 := f.pin(f.manager.MkArithmeticLE(f.pin(f.manager.MkArithmeticAdd(f.pin(f.manager.MkArithmeticMul(neg, f.x)), neg)), z))
	form := f.pin(f.manager.MkAnd(f.pin(f.manager.MkAnd(l0, l1)), a))
	ok, e := f.plugin.Prepare(f.contains, form, []ast.ASTHandle{l0, l1, a}, nil)
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
	r, e := f.manager.NewRef(form)
	pluginMust(t, e)
	defer f.manager.DelRef(&r)
	d, e := f.manager.NewRef(ast.ASTHandle{})
	pluginMust(t, e)
	defer f.manager.DelRef(&d)
	pluginMust(t, f.plugin.Substitute(f.contains, &v, r, &d))
	if h, e := f.manager.RefGet(d); e != nil || h.IsNull() {
		t.Fatal(h, e)
	}
}
