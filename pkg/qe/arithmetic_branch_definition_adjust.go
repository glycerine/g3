package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// adjustDefinition is get_def's x_t adjustment: t = t + a*m_t and
// out = a * m_coeff (a itself without a substitution).
func (p *ArithmeticPlugin) adjustDefinition(t ast.ASTHandle, a *unumeral.Rational, s *arithmeticBranchSubstitution, out *unumeral.Rational) (ast.ASTHandle, error) {
	q := p.util.q()
	if s == nil {
		return t, q.Copy(out, a)
	}
	r, e := p.util.scale(a, s.term)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	h, e := p.manager.RefGet(r)
	if e != nil {
		_ = p.manager.DelRef(&r)
		return ast.ASTHandle{}, e
	}
	v, e := p.manager.MkArithmeticAdd(t, h)
	e = errors.Join(e, p.manager.DelRef(&r))
	if e != nil {
		return v, e
	}
	return v, q.Mul(a, &s.coefficient, out)
}
