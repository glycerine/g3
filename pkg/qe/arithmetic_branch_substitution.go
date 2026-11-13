package qe

// Source: src/qe/qe_arith_plugin.cpp x_subst.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// arithmeticBranchSubstitution is x_subst: x |-> coefficient*x + term. A nil
// substitution is the default x_subst (no term, coefficient one).
type arithmeticBranchSubstitution struct {
	coefficient *big.Rat
	term        ast.ASTHandle
}

// mkTerm is x_subst::mk_term: t + c*m_t, built as mk_add(mk_mul(c, m_t), t)
// (mk_mul returns m_t for a unit c); t itself without a term.
func (x *arithmeticBranchSubstitution) mkTerm(p *ArithmeticPlugin, c *big.Rat, t ast.ASTHandle) (ast.ASTHandle, error) {
	if x == nil || x.term.IsNull() {
		return t, nil
	}
	m := p.manager
	scaled := x.term
	if c.Cmp(big.NewRat(1, 1)) != 0 {
		numeral, err := p.util.numeralFor(c, x.term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if scaled, err = m.MkArithmeticMul(numeral, x.term); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return m.MkArithmeticAdd(scaled, t)
}

// mkCoeff is x_subst::mk_coeff: k * m_coeff.
func (x *arithmeticBranchSubstitution) mkCoeff(k *big.Rat) *big.Rat {
	if x == nil {
		return new(big.Rat).Set(k)
	}
	return new(big.Rat).Mul(k, x.coefficient)
}
