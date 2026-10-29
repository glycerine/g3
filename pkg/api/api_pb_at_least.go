package api

import "github.com/glycerine/g3/pkg/ast"

// PseudoBooleanAtLeast constructs a cardinality constraint with at least k
// true arguments. The returned handle is borrowed from the context manager.
func (c *SMTContext) PseudoBooleanAtLeast(args []ast.ASTHandle, k uint32) (ast.ASTHandle, error) {
	manager, err := c.pseudoBooleanManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkPseudoBooleanAtLeast(args, k)
}
