package opt

// Source: src/opt/opt_context.cpp is_maxsat negative-weight normalization and
// src/opt/maxsmt.cpp maxsmt::add duplicate accumulation.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Add borrows weight; the set keeps its own copy.
func (s *weightedSoftSet) Add(term ast.ASTHandle, weight *numeral.Rational) error {
	if s == nil || s.closed || s.manager == nil || weight == nil {
		return ast.ErrASTHandle
	}
	o := newRatOps(s.manager)
	if o.sign(weight) == 0 {
		return o.err
	}
	normalized := term
	positive := o.clone(weight)
	if o.sign(&positive) < 0 {
		o.add(&s.offset, &s.offset, &positive)
		o.neg(&positive)
		if !o.ok() {
			o.del(&positive)
			return o.err
		}
		var err error
		normalized, err = s.manager.MkNot(term)
		if err != nil {
			o.del(&positive)
			return o.joined(err)
		}
	}
	if index, found := s.index[normalized]; found {
		o.add(&s.weights[index], &s.weights[index], &positive)
		o.del(&positive)
		return o.err
	}
	if !o.ok() {
		o.del(&positive)
		return o.err
	}
	ref, err := s.manager.NewRef(normalized)
	if err != nil {
		o.del(&positive)
		return o.joined(err)
	}
	s.index[normalized] = len(s.terms)
	s.terms = append(s.terms, ref)
	s.weights = append(s.weights, positive)
	return nil
}
