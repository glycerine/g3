package bv

// Source: src/ast/rewriter/bv_bounds_base.h zero_patch.

import (
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

func (s *BVBoundsSimplifier) zeroPatch(h ast.ASTHandle, out ast.ASTRef) (bool, error) {
	m := s.manager
	kind, e := m.GetKind(h)
	if e != nil || kind != ast.ASTApp {
		return false, e
	}
	n, e := m.GetNumArgs(h)
	if e != nil || n == 0 {
		return false, e
	}
	args := make([]ast.ASTHandle, n)
	changed := false
	for i := uint32(0); i < n; i++ {
		arg, x := m.GetArg(h, i)
		if x != nil {
			return false, x
		}
		args[i] = arg
		b, ok := s.bounds[arg]
		if !ok || len(b.ranges) != 1 || b.ranges[0].lo.Cmp(b.ranges[0].hi) > 0 {
			continue
		}
		var low, high uint32
		var base ast.ASTHandle
		if extract, x := m.BitVectorExtractValue(arg, &low, &high, &base); x != nil {
			return false, x
		} else if extract {
			continue
		}
		width, x := m.BitVectorSize(arg)
		if x != nil {
			continue
		}
		bits := uint32(b.ranges[0].hi.BitLen())
		if bits == 0 || bits >= width {
			continue
		}
		zero, x := bvBoundsMakeNumeral(m, big.NewInt(0), width-bits)
		if x != nil {
			return false, x
		}
		part, x := m.MkBitVectorExtract(bits-1, 0, arg)
		if x != nil {
			return false, x
		}
		args[i], x = m.MkBitVectorConcatPair(zero, part)
		if x != nil {
			return false, x
		}
		changed = true
	}
	if !changed {
		return false, nil
	}
	decl, e := m.GetDecl(h)
	if e != nil {
		return false, e
	}
	value, e := m.MkApp(decl, args)
	if e == nil {
		e = m.SetRef(out, value)
	}
	return e == nil, e
}
