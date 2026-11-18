package qe

// Source: src/qe/qe_arith_plugin.cpp bound.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type ArithmeticBoundDirection uint8

const (
	ArithmeticLowerBound ArithmeticBoundDirection = iota
	ArithmeticUpperBound
)

// ArithmeticBound owns Term, Atom and Coefficient (bounds_proc's
// m_*_coeffs entry, a rational of the manager's NumeralService().Manager).
type ArithmeticBound struct {
	manager     *ast.ASTManager
	Term        ast.ASTRef
	Atom        ast.ASTRef
	Coefficient unumeral.Rational
	Strict      bool
	Direction   ArithmeticBoundDirection
}

func (b *ArithmeticBound) Close() error {
	if b == nil || b.manager == nil {
		return ast.ErrASTHandle
	}
	err := errors.Join(b.manager.DelRef(&b.Atom), b.manager.DelRef(&b.Term), b.manager.NumeralService().Manager.Del(&b.Coefficient))
	if err == nil {
		b.manager = nil
	}
	return err
}
