package cmd_context

// Source: src/muz/base/dl_context.cpp context::get_answer_as_formula.

import "github.com/glycerine/g3/pkg/ast"

// FixedpointExpressionAnswer returns an owning answer for the latest
// expression query. The caller releases it through Manager.DelRef.
func (c *Context) FixedpointExpressionAnswer() (ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return engine.ExpressionAnswer()
}
