package bv

// Source: src/ast/simplifiers/bv_slice.h state and apply_subst temporaries.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"sort"
)

type bvSliceFrame struct {
	owner      *BVSliceTactic
	boundaries map[ast.ASTHandle]map[uint32]struct{}
	keys       map[ast.ASTHandle]ast.ASTRef
	refs       []ast.ASTRef
	keyOrder   []ast.ASTHandle // keys in insertion order, released forward
}

func newBVSliceFrame(t *BVSliceTactic) *bvSliceFrame {
	return &bvSliceFrame{owner: t, boundaries: make(map[ast.ASTHandle]map[uint32]struct{}), keys: make(map[ast.ASTHandle]ast.ASTRef)}
}
func (f *bvSliceFrame) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, e := f.owner.manager.NewRef(h)
	if e == nil {
		f.refs = append(f.refs, r)
	}
	return h, e
}
func (f *bvSliceFrame) pinKey(h ast.ASTHandle) error {
	if _, ok := f.keys[h]; ok {
		return nil
	}
	r, e := f.owner.manager.NewRef(h)
	if e == nil {
		f.keys[h] = r
		f.keyOrder = append(f.keyOrder, h)
	}
	return e
}
func (f *bvSliceFrame) points(h ast.ASTHandle) []uint32 {
	set := f.boundaries[h]
	r := make([]uint32, 0, len(set))
	for v := range set {
		r = append(r, v)
	}
	sort.Slice(r, func(i, j int) bool { return r[i] < r[j] })
	return r
}
func (f *bvSliceFrame) Close() (err error) {
	for i := len(f.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, f.owner.manager.DelRef(&f.refs[i]))
	}
	for _, h := range f.keyOrder {
		if r, held := f.keys[h]; held {
			err = errors.Join(err, f.owner.manager.DelRef(&r))
			delete(f.keys, h)
		}
	}
	f.keyOrder = nil
	f.refs = nil
	f.boundaries = nil
	return err
}
