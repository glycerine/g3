package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type bvModelProjectFixture struct {
	*bvPluginFixture
	model   *model.Model
	formula ast.ASTRef
}

func newBVModelProjectFixture(t *testing.T, width uint32, value uint64) *bvModelProjectFixture {
	t.Helper()
	f := newBVPluginFixture(t, width)
	md, err := model.NewModel(f.manager)
	fmMust(t, err)
	decl, err := f.manager.GetDecl(f.variable)
	fmMust(t, err)
	interpreted := f.base.own(f.manager.BitVectorNumeralUint64(value, width))
	fmMust(t, md.Core().RegisterConstant(decl, interpreted))
	formula, err := f.manager.NewRef(f.variable)
	fmMust(t, err)
	t.Cleanup(func() {
		fmMust(t, f.manager.DelRef(&formula))
		fmMust(t, md.Close())
	})
	return &bvModelProjectFixture{bvPluginFixture: f, model: md, formula: formula}
}

func (f *bvModelProjectFixture) project(t *testing.T) ast.ASTHandle {
	t.Helper()
	projected, err := f.plugin.ProjectModel(f.contains, f.model, f.formula, f.base.r.config)
	fmMust(t, err)
	if !projected {
		t.Fatal("bit-vector model projection declined")
	}
	result, err := f.manager.RefGet(f.formula)
	fmMust(t, err)
	return result
}
