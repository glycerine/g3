package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

type bvBoundsFixture struct {
	base    *BVSizeReductionTactic
	m       *ast.ASTManager
	simp    *BVBoundsSimplifier
	sort, x ast.ASTHandle
	pins    []ast.ASTRef
}

func newBVBoundsFixture(t *testing.T) *bvBoundsFixture {
	t.Helper()
	base := bvSizeFixture(t, false)
	m := base.manager
	sharingMust(t, m.RegisterDeclarationPlugins())
	sort, e := m.BitVectorSort(8)
	sharingMust(t, e)
	name, e := m.Symbols.Intern("bounds-x")
	sharingMust(t, e)
	x, e := m.MkConst(name, sort)
	sharingMust(t, e)
	f := &bvBoundsFixture{base: base, m: m, sort: sort, x: x}
	f.simp = NewBVBoundsSimplifier(m, base.context)
	t.Cleanup(func() {
		sharingMust(t, f.simp.Close())
		for i := range f.pins {
			sharingMust(t, m.DelRef(&f.pins[i]))
		}
	})
	f.pin(t, x)
	return f
}
func (f *bvBoundsFixture) pin(t *testing.T, h ast.ASTHandle) ast.ASTHandle {
	r, e := f.m.NewRef(h)
	sharingMust(t, e)
	f.pins = append(f.pins, r)
	return h
}
func (f *bvBoundsFixture) num(t *testing.T, n uint32) ast.ASTHandle {
	h, e := f.m.BitVectorNumeralUnsigned(n, 8)
	sharingMust(t, e)
	return f.pin(t, h)
}
func (f *bvBoundsFixture) ule(t *testing.T, a, b ast.ASTHandle) ast.ASTHandle {
	h, e := f.m.MkBitVectorULE(a, b)
	sharingMust(t, e)
	return f.pin(t, h)
}
func (f *bvBoundsFixture) sle(t *testing.T, a, b ast.ASTHandle) ast.ASTHandle {
	h, e := f.m.MkBitVectorSLE(a, b)
	sharingMust(t, e)
	return f.pin(t, h)
}
