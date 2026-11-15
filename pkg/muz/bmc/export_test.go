package bmc

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// CompileQuantifiedLinearEmit exposes qlinear::compile's per-formula
// assert_expr path to the external tests.
func (c *Compiler) CompileQuantifiedLinearEmit(rules *base.RuleSet, width uint32, emit func(ast.ASTHandle) error) error {
	_, err := c.compileQuantifiedLinear(rules, width, emit)
	return err
}
