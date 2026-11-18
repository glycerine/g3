package arith

// Source: src/tactic/arith/fm_tactic.cpp:879-903.
import (
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (t *FMExpressionTable) constraintPolynomial(c *FMConstraint) (ast.ASTHandle, ast.ASTHandle, error) {
	integer := t.variables.AllInteger(c)
	terms := make([]ast.ASTHandle, 0, len(c.Variables))
	for i, v := range c.Variables {
		x, err := t.Variable(v)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		if !integer && t.variables.IsInteger(v) {
			family, err := t.manager.RegisterArithmeticPlugin()
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
			x, err = t.manager.MkFamilyApp(family, 14, nil, []ast.ASTHandle{x})
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
		}
		if !c.Coefficients[i].IsOne() {
			numeral, err := t.numeral(&c.Coefficients[i], integer)
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
			x, err = t.manager.MkArithmeticMul(numeral, x)
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
		}
		terms = append(terms, x)
	}
	lhs := terms[0]
	if len(terms) > 1 {
		family, err := t.manager.RegisterArithmeticPlugin()
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		lhs, err = t.manager.MkFamilyApp(family, 6, nil, terms)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
	}
	rhs, err := t.numeral(&c.Constant, integer)
	return lhs, rhs, err
}

// numeral is a.mk_numeral(v, integer).
func (t *FMExpressionTable) numeral(v *unumeral.Rational, integer bool) (ast.ASTHandle, error) {
	return t.manager.ArithmeticNumeralFromRational(v, integer)
}
