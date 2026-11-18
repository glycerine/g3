package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util::mk_bounded_var.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// mkBoundedVar is arith_qe_util::mk_bounded_var: a fresh bit-vector z_bv
// wide enough for n and its unsigned integer conversion z = (bv2int z_bv).
// It returns z unreferenced; z holds z_bv (get_arg(0)).
func (u *ArithmeticQETermUtil) mkBoundedVar(n *unumeral.Rational) (result ast.ASTHandle, resultErr error) {
	if n == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	q := u.q()
	// rational two(2), b(n);
	two, b := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&two), q.Del(&b)) }()
	if err := errors.Join(q.SetInt32(&two, 2), q.Copy(&b, n)); err != nil {
		return ast.ASTHandle{}, err
	}
	// do { ++sz; b = div(b, two); } while (b.is_pos());
	width := uint32(0)
	for {
		width++
		if err := q.IDiv(&b, &two, &b); err != nil {
			return ast.ASTHandle{}, err
		}
		positive, err := q.IsPos(&b)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !positive {
			break
		}
	}
	m := u.manager
	if _, err := m.RegisterBitVectorPlugin(); err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := m.BitVectorSort(width)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bits, err := m.MkFreshConst("z", sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.MkBitVectorToInt(bits)
}
