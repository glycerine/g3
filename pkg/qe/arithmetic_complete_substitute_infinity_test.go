package qe_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArithmeticAssignSubstituteCompleteInfinity(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	truth := f.pin(f.manager.MkTrue())
	ok, e := f.plugin.Prepare(f.contains, truth, nil, nil)
	pluginMust(t, e)
	if !ok {
		t.Fatal("prepare")
	}
	s := f.manager.NumeralService()
	v, e := s.NewValue()
	pluginMust(t, e)
	defer s.Del(&v)
	pluginMust(t, s.SetUnsigned(&v, 0))
	r, e := f.manager.NewRef(truth)
	pluginMust(t, e)
	defer f.manager.DelRef(&r)
	d, e := f.manager.NewRef(ast.ASTHandle{})
	pluginMust(t, e)
	defer f.manager.DelRef(&d)
	pluginMust(t, f.plugin.AssignSubstituteComplete(f.contains, r, &v, &d))
	h, e := f.manager.RefGet(d)
	pluginMust(t, e)
	if h.IsNull() {
		t.Fatal("missing infinity definition")
	}
}
