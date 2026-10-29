package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

type bvSliceFixture struct {
	base       *BVSizeReductionTactic
	tactic     *BVSliceTactic
	m          *ast.ASTManager
	sort, x, y ast.ASTHandle
	pins       []ast.ASTRef
}

func newBVSliceFixture(t *testing.T) *bvSliceFixture {
	t.Helper()
	base := bvSizeFixture(t, false)
	m := base.manager
	sharingMust(t, m.RegisterDeclarationPlugins())
	runner, e := NewBVSliceTactic(m, base.context, base.globals, nil, base.services)
	sharingMust(t, e)
	sort, e := m.BitVectorSort(32)
	sharingMust(t, e)
	f := &bvSliceFixture{base: base, tactic: runner, m: m, sort: sort}
	for _, item := range []struct {
		name string
		dst  *ast.ASTHandle
	}{{"slice-x", &f.x}, {"slice-y", &f.y}} {
		name, e := m.Symbols.Intern(item.name)
		sharingMust(t, e)
		*item.dst, e = m.MkConst(name, sort)
		sharingMust(t, e)
		f.pin(t, *item.dst)
	}
	t.Cleanup(func() {
		sharingMust(t, runner.Close())
		for i := range f.pins {
			sharingMust(t, m.DelRef(&f.pins[i]))
		}
	})
	return f
}
func (f *bvSliceFixture) pin(t *testing.T, h ast.ASTHandle) ast.ASTHandle {
	r, e := f.m.NewRef(h)
	sharingMust(t, e)
	f.pins = append(f.pins, r)
	return h
}
func (f *bvSliceFixture) extract(t *testing.T, hi, lo uint32, h ast.ASTHandle) ast.ASTHandle {
	v, e := f.m.MkBitVectorExtract(hi, lo, h)
	sharingMust(t, e)
	return f.pin(t, v)
}
func (f *bvSliceFixture) eq(t *testing.T, a, b ast.ASTHandle) ast.ASTHandle {
	v, e := f.m.MkEq(a, b)
	sharingMust(t, e)
	return f.pin(t, v)
}
