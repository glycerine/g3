package opt

// Source: src/opt/maxsmt.cpp expression-vector lifetime and objective handoff.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Release transfers the terms, weights and offset to the caller.
func (s *weightedSoftSet) Release() (terms []ast.ASTRef, weights []numeral.Rational, offset numeral.Rational, err error) {
	offset = numeral.NewRational()
	if s == nil || s.closed || s.manager == nil {
		return nil, nil, offset, ast.ErrASTHandle
	}
	terms, weights, offset = s.terms, s.weights, s.offset
	s.terms = nil
	s.weights = nil
	s.offset = numeral.NewRational()
	s.index = nil
	s.closed = true
	s.manager = nil
	return terms, weights, offset, nil
}

func (s *weightedSoftSet) Close() (result error) {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	o := newRatOps(s.manager)
	for i := len(s.terms) - 1; i >= 0; i-- {
		result = errors.Join(result, s.manager.DelRef(&s.terms[i]))
	}
	o.delAll(s.weights)
	o.del(&s.offset)
	s.terms = nil
	s.weights = nil
	s.index = nil
	s.manager = nil
	return o.joined(result)
}
