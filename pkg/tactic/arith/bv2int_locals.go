package arith

// Source: src/tactic/arith/bv2int_rewriter.cpp mk_bv_add and align_sizes,
// whose expr_ref locals release a replaced value at the assignment.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// newLocal is expr_ref x(h, m()).
func (c *bv2IntCall) newLocal(h ast.ASTHandle) ast.ASTRef {
	if c.err != nil {
		return ast.ASTRef{}
	}
	r, err := c.r.manager.NewRef(h)
	c.err = err
	return r
}

func (c *bv2IntCall) localValue(r ast.ASTRef) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	h, err := c.r.manager.RefGet(r)
	c.err = err
	return h
}

// assignLocal is `x = h` for a raw result h of a helper: the local takes h
// and releases its previous value; the frame's hold that kept h is dropped.
func (c *bv2IntCall) assignLocal(r *ast.ASTRef, h ast.ASTHandle) {
	if c.err != nil {
		return
	}
	if c.localValue(*r) == h || c.err != nil {
		return
	}
	if c.err = c.r.manager.SetRef(*r, h); c.err != nil {
		return
	}
	c.release(h)
}

// releaseLocals runs the expr_ref destructors, in the given order.
func (c *bv2IntCall) releaseLocals(locals ...*ast.ASTRef) {
	for _, r := range locals {
		if *r != (ast.ASTRef{}) {
			c.err = errors.Join(c.err, c.r.manager.DelRef(r))
		}
	}
}

// alignLocals is align_sizes(expr_ref& s, expr_ref& t, is_signed).
func (c *bv2IntCall) alignLocals(s, t *ast.ASTRef, signed bool) {
	a, b := c.width(c.localValue(*s)), c.width(c.localValue(*t))
	if c.err != nil {
		return
	}
	if a > b {
		c.assignLocal(t, c.extend(a-b, c.localValue(*t), signed))
	}
	if a < b {
		c.assignLocal(s, c.extend(b-a, c.localValue(*s), signed))
	}
}
