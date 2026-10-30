package bv

// Source: src/ast/rewriter/bv_bounds_base.h simplify_core/may_simplify.

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (s *BVBoundsSimplifier) MaySimplify(h ast.ASTHandle) (bool, error) {
	if s == nil || s.manager == nil || h.IsNull() {
		return false, ast.ErrASTDomain
	}
	if _, _, numeral, err := bvBoundsNumeral(s.manager, h); err != nil || numeral {
		return false, err
	}
	for {
		not, argument, err := s.manager.IsNot(h)
		if err != nil {
			return false, err
		}
		if !not {
			break
		}
		h = argument
	}
	for variable := range s.bounds {
		contains, err := s.containsExpression(h, variable)
		if err != nil || contains {
			return contains, err
		}
	}
	variable, bound, found, err := s.recognize(h)
	if err != nil {
		return false, err
	}
	if found {
		_, contextual := s.bounds[variable]
		return bound.full() || contextual, nil
	}
	return s.containsBound(h)
}
func (s *BVBoundsSimplifier) Simplify(h ast.ASTHandle, out ast.ASTRef) (bool, error) {
	if b, ok := s.bounds[h]; ok {
		if value, single := b.singleton(); single {
			v, e := bvBoundsMakeNumeral(s.manager, value, b.width)
			if e != nil {
				return false, e
			}
			return true, s.manager.SetRef(out, v)
		}
	}
	if done, e := s.zeroPatch(h, out); e != nil || done {
		return done, e
	}
	key, want, ok, e := s.recognize(h)
	if e != nil || !ok {
		return false, e
	}
	have, exists := s.bounds[key]
	if !exists {
		return false, nil
	}
	var value ast.ASTHandle
	if have.implies(want) {
		value, e = s.manager.MkTrue()
	} else if have.intersect(want).empty() {
		value, e = s.manager.MkFalse()
	} else if s.propagateEq {
		if singleton, yes := have.intersect(want).singleton(); yes {
			n, x := bvBoundsMakeNumeral(s.manager, singleton, want.width)
			if x != nil {
				return false, x
			}
			value, e = s.manager.MkEq(key, n)
		}
	}
	if e != nil || value.IsNull() {
		return false, e
	}
	return true, s.manager.SetRef(out, value)
}
