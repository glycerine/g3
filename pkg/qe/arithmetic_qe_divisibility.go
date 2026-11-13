package qe

// Source: src/qe/qe_arith_plugin.cpp div_constraint/bounds_proc divisibility.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// ArithmeticDivisibility owns Term, Atom and, for a nested modulo term, Z:
// the bounded residue (bv2int z_bv) bounds_proc made when it found the term
// (m_nested_div_z).
type ArithmeticDivisibility struct {
	manager     *ast.ASTManager
	Term        ast.ASTRef
	Atom        ast.ASTRef
	Z           ast.ASTRef
	Coefficient *big.Rat
	Divisor     *big.Rat
	Negated     bool
	Nested      bool
}

func (d *ArithmeticDivisibility) Close() error {
	if d == nil || d.manager == nil {
		return ast.ErrASTHandle
	}
	var err error
	if d.Z != (ast.ASTRef{}) {
		err = d.manager.DelRef(&d.Z)
	}
	err = errors.Join(err, d.manager.DelRef(&d.Atom), d.manager.DelRef(&d.Term))
	if err == nil {
		d.manager = nil
	}
	return err
}
