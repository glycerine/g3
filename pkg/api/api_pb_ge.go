package api

import "github.com/glycerine/g3/pkg/ast"

// PseudoBooleanGE constructs sum(coefficients[i]*args[i]) >= bound.
func (c *SMTContext) PseudoBooleanGE(args []ast.ASTHandle, coefficients []int32, bound int32) (ast.ASTHandle, error) {
	manager, err := c.pseudoBooleanManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkPseudoBooleanGEIntegers(coefficients, args, bound)
}
