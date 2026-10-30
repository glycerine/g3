package cmd_context

// Source: src/api/api_datalog.cpp:347-358 and
// src/muz/base/dl_context.cpp display_smt2.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// WriteFixedpointSMT2WithQueries writes the current fixedpoint problem and
// appends query expressions in caller order.
func (c *Context) WriteFixedpointSMT2WithQueries(out io.Writer, queries []ast.ASTHandle) error {
	if out == nil {
		return ast.ErrASTHandle
	}
	if err := c.WriteFixedpointSMT2(out); err != nil {
		return err
	}
	for _, query := range queries {
		if _, err := io.WriteString(out, "(query "); err != nil {
			return err
		}
		if err := c.WriteExpression(out, query); err != nil {
			return err
		}
		if _, err := io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	return nil
}
