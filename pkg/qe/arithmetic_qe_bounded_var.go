package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util::mk_bounded_var.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// mkBoundedVar is arith_qe_util::mk_bounded_var: a fresh bit-vector z_bv
// wide enough for n and its unsigned integer conversion z = (bv2int z_bv).
// It returns z unreferenced; z holds z_bv (get_arg(0)).
func (u *ArithmeticQETermUtil) mkBoundedVar(n *big.Rat) (ast.ASTHandle, error) {
	if n == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	// do { ++sz; b = div(b, 2); } while (b.is_pos());
	b := new(big.Int).Quo(n.Num(), n.Denom())
	if n.Sign() < 0 && !n.IsInt() {
		b.Sub(b, big.NewInt(1))
	}
	width := uint32(1)
	if b.Sign() > 0 {
		if uint64(b.BitLen()) > uint64(^uint32(0)) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		width = uint32(b.BitLen())
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
