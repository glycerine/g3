package qe

// Source: src/qe/qe_bv_plugin.cpp project evaluation and numeral extraction.

import "github.com/glycerine/g3/pkg/ast"

func (c *bitVectorModelProjectCall) evaluate(target ast.ASTHandle) error {
	if err := c.evaluator.Evaluate(target, c.value); err != nil {
		return err
	}
	evaluated, err := c.plugin.manager.RefGet(c.value)
	if err != nil {
		return err
	}
	matched, err := c.plugin.manager.BitVectorNumeralValue(evaluated, &c.numeral, &c.width)
	if err != nil {
		return err
	}
	if !matched {
		return ast.ErrASTDomain
	}
	targetWidth, err := c.plugin.manager.BitVectorSize(target)
	if err != nil {
		return err
	}
	if c.width != targetWidth {
		return ast.ErrASTDomain
	}
	return nil
}
