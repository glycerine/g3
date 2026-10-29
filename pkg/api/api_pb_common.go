package api

// Source: src/api/api_pb.cpp public pseudo-Boolean constructors.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) pseudoBooleanManager() (*ast.ASTManager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.Manager()
}
