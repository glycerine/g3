package spacer

// Source: src/muz/spacer/spacer_context.cpp context::solve wraps every false
// solve_core result with frame simplification and optional result validation.

import "github.com/glycerine/g3/pkg/util/lbool"

func (c *Context) finishEarlyUnreachable(inductive uint32) (lbool.LBool, error) {
	c.solved = true
	c.inductive = inductive
	c.lastResult = lbool.False
	if err := c.simplifyFormulas(); err != nil {
		return c.lastResult, err
	}
	return c.lastResult, c.validate()
}
