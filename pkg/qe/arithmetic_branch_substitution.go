package qe

// Source: src/qe/qe_arith_plugin.cpp x_subst.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// arithmeticBranchSubstitution is x_subst: x |-> coefficient*x + term. A nil
// substitution is the default x_subst (no term, coefficient one). The
// coefficient is m_coeff, a rational of the manager's
// NumeralService().Manager owned by the substitution (release).
type arithmeticBranchSubstitution struct {
	coefficient unumeral.Rational
	term        ast.ASTHandle
}

// newArithmeticBranchSubstitution is x_subst::set_coeff/set_term with a copy
// of coefficient.
func newArithmeticBranchSubstitution(q *unumeral.RationalManager, coefficient *unumeral.Rational, term ast.ASTHandle) (*arithmeticBranchSubstitution, error) {
	x := &arithmeticBranchSubstitution{coefficient: unumeral.NewRational(), term: term}
	if err := q.Copy(&x.coefficient, coefficient); err != nil {
		return nil, errors.Join(err, q.Del(&x.coefficient))
	}
	return x, nil
}

// release is the x_subst destructor (m_coeff).
func (x *arithmeticBranchSubstitution) release(q *unumeral.RationalManager) error {
	if x == nil {
		return nil
	}
	return q.Del(&x.coefficient)
}

// mkTerm is x_subst::mk_term: t + c*m_t, built as mk_add(mk_mul(c, m_t), t)
// (mk_mul returns m_t for a unit c); t itself without a term.
func (x *arithmeticBranchSubstitution) mkTerm(p *ArithmeticPlugin, c *unumeral.Rational, t ast.ASTHandle) (ast.ASTHandle, error) {
	if x == nil || x.term.IsNull() {
		return t, nil
	}
	m := p.manager
	scaled := x.term
	one, err := p.util.q().IsOne(c)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !one {
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

// mkCoeff is x_subst::mk_coeff: out = k * m_coeff (k itself without a
// substitution).
func (x *arithmeticBranchSubstitution) mkCoeff(q *unumeral.RationalManager, k, out *unumeral.Rational) error {
	if x == nil {
		return q.Copy(out, k)
	}
	return q.Mul(k, &x.coefficient, out)
}
