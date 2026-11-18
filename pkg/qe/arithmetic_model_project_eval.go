package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c *arithmeticModelProjectCall) evaluate(expression ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.evaluator.Evaluate(expression, c.value); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.plugin.manager.RefGet(c.value)
}

func (c *arithmeticModelProjectCall) truth(expression ast.ASTHandle) (bool, error) {
	value, err := c.evaluate(expression)
	return err == nil && c.plugin.manager.IsTrue(value), err
}

// numeral is eval(e, vl); VERIFY(m_arith.is_numeral(vl, num)), writing num
// (a rational of the manager's NumeralService().Manager).
func (c *arithmeticModelProjectCall) numeral(expression ast.ASTHandle, num *unumeral.Rational) error {
	value, err := c.evaluate(expression)
	if err != nil {
		return err
	}
	matched, err := c.plugin.util.numeral(value, num)
	if err != nil {
		return err
	}
	if !matched {
		return ast.ErrASTDomain
	}
	return nil
}
