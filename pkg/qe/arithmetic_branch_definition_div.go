package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (p *ArithmeticPlugin) definitionDivision(t ast.ASTHandle, a *unumeral.Rational) (ast.ASTHandle, error) {
	n, e := p.manager.MkArithmeticNeg(t)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if one, e := p.util.q().IsOne(a); e != nil {
		return ast.ASTHandle{}, e
	} else if one {
		return n, nil
	}
	d, e := p.util.numeralFor(a, t)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return p.manager.MkArithmeticDiv(n, d)
}
