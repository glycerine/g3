package api

// Source: src/api/api_arith.cpp:177-198.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (c *SMTContext) arithmeticRationalPart(a ast.ASTHandle, numerator bool) (ast.ASTHandle, error) {
	m, _, e := c.arithmeticManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	// rational val; is_numeral(e, val); mk_numeral(numerator(val), true)
	var result ast.ASTHandle
	e = withRationals(m, 2, func(q *numeral.RationalManager, rs []numeral.Rational) error {
		_, yes, err := m.ArithmeticNumeralRational(a, &rs[0])
		if err != nil {
			return err
		}
		if !yes {
			return ast.ErrASTDomain
		}
		if numerator {
			err = q.NumeratorRational(&rs[0], &rs[1])
		} else {
			err = q.DenominatorRational(&rs[0], &rs[1])
		}
		if err == nil {
			result, err = m.ArithmeticNumeralFromRational(&rs[1], true)
		}
		return err
	})
	return result, e
}
func (c *SMTContext) Numerator(a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticRationalPart(a, true)
}
func (c *SMTContext) Denominator(a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.arithmeticRationalPart(a, false)
}
