package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type bvPluginFixture struct {
	t        *testing.T
	base     *fmFixture
	manager  *ast.ASTManager
	context  *SolverContext
	plugin   *BitVectorPlugin
	sort     ast.ASTHandle
	variable ast.ASTHandle
	contains *ast.ContainsApp
	formula  ast.ASTHandle
}

func newBVPluginFixture(t *testing.T, width uint32) *bvPluginFixture {
	t.Helper()
	base := newFMFixture(t)
	m := base.r.manager
	_, err := m.RegisterBitVectorPlugin()
	fmMust(t, err)
	sort := base.own(m.BitVectorSort(width))
	x := base.constant("qe-bv-x", sort)
	y := base.constant("qe-bv-y", sort)
	formula := base.own(m.MkEq(x, y))
	contains, err := ast.NewContainsApp(m, x)
	fmMust(t, err)
	context, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	plugin, err := NewBitVectorPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(plugin))
	f := &bvPluginFixture{t: t, base: base, manager: m, context: context, plugin: plugin, sort: sort, variable: x, contains: contains, formula: formula}
	t.Cleanup(func() {
		fmMust(t, context.Close())
		fmMust(t, contains.Close())
	})
	return f
}

func (f *bvPluginFixture) rational(text string) *numeral.OwnedRational {
	f.t.Helper()
	service := f.manager.NumeralService()
	value, err := service.NewValue()
	fmMust(f.t, err)
	fmMust(f.t, service.SetString(&value, text))
	f.t.Cleanup(func() { fmMust(f.t, service.CleanupValue(&value)) })
	return &value
}
