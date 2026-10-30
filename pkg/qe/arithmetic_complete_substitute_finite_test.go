package qe_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArithmeticAssignSubstituteCompleteFinite(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	z := f.integerNumeral(t, "0")
	n := f.integerNumeral(t, "-1")
	l0 := f.pin(f.manager.MkArithmeticLE(f.pin(f.manager.MkArithmeticMul(n, f.x)), z))
	l1 := f.pin(f.manager.MkArithmeticLE(f.pin(f.manager.MkArithmeticAdd(f.pin(f.manager.MkArithmeticMul(n, f.x)), n)), z))
	u := f.pin(f.manager.MkArithmeticLE(f.pin(f.manager.MkArithmeticAdd(f.x, n)), z))
	form := f.pin(f.manager.MkAnd(f.pin(f.manager.MkAnd(l0, l1)), u))
	ok, e := f.plugin.Prepare(f.contains, form, []ast.ASTHandle{l0, l1, u}, nil)
	pluginMust(t, e)
	if !ok {
		t.Fatal("prepare")
	}
	s := f.manager.NumeralService()
	v, e := s.NewValue()
	pluginMust(t, e)
	defer s.Del(&v)
	pluginMust(t, s.SetUnsigned(&v, 1))
	r, e := f.manager.NewRef(form)
	pluginMust(t, e)
	defer f.manager.DelRef(&r)
	d, e := f.manager.NewRef(ast.ASTHandle{})
	pluginMust(t, e)
	defer f.manager.DelRef(&d)
	pluginMust(t, f.plugin.AssignSubstituteComplete(f.contains, r, &v, &d))
	h, e := f.manager.RefGet(d)
	pluginMust(t, e)
	if h.IsNull() {
		t.Fatal("missing finite definition")
	}
}
