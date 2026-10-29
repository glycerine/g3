package api

import "github.com/glycerine/g3/pkg/ast"

// PseudoBooleanAtMost constructs a cardinality constraint with at most k true
// arguments. The returned handle is borrowed from the context manager.
func (c *SMTContext) PseudoBooleanAtMost(args []ast.ASTHandle, k uint32) (ast.ASTHandle, error) {
	manager, err := c.pseudoBooleanManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkPseudoBooleanAtMost(args, k)
}
