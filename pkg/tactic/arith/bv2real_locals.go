package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp bv2real_util::mk_bv_mul,
// mk_bv_add and align_sizes, whose expr_ref locals s1 and t1 release a
// replaced value at the assignment.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// newLocal is expr_ref x(h, m()).
func (c *bv2RealCall) newLocal(h ast.ASTHandle) ast.ASTRef {
	if c.err != nil {
		return ast.ASTRef{}
	}
	r, err := c.u.manager.NewRef(h)
	c.err = err
	return r
}

// localValue reads an expr_ref local.
func (c *bv2RealCall) localValue(r ast.ASTRef) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	h, err := c.u.manager.RefGet(r)
	c.err = err
	return h
}

// assignLocal is `x = h` for a raw result h: the local takes h, releasing
// its previous value, and the scope hold that made h escape from a helper
// (at or above mark) is dropped, so h is owned by the local alone.
func (c *bv2RealCall) assignLocal(mark bv2RealCallCheckpoint, r *ast.ASTRef, h ast.ASTHandle) {
	if c.err != nil {
		return
	}
	if c.localValue(*r) == h || c.err != nil {
		return
	}
	if c.err = c.u.manager.SetRef(*r, h); c.err != nil {
		return
	}
	c.dropHold(mark.refs, h)
}

// dropHold releases the latest scope hold of h at or above index from.
func (c *bv2RealCall) dropHold(from int, h ast.ASTHandle) {
	for i := len(c.refs) - 1; i >= from && c.err == nil; i-- {
		held, err := c.u.manager.RefGet(c.refs[i])
		if err != nil {
			c.err = err
			return
		}
		if held != h {
			continue
		}
		if c.err = c.u.manager.DelRef(&c.refs[i]); c.err != nil {
			return
		}
		copy(c.refs[i:], c.refs[i+1:])
		c.refs[len(c.refs)-1] = ast.ASTRef{}
		c.refs = c.refs[:len(c.refs)-1]
		return
	}
}

// releaseLocals runs the expr_ref destructors, in the given order.
func (c *bv2RealCall) releaseLocals(locals ...*ast.ASTRef) {
	for _, r := range locals {
		if *r != (ast.ASTRef{}) {
			c.err = errors.Join(c.err, c.u.manager.DelRef(r))
		}
	}
}

// alignLocals is align_sizes(expr_ref& s, expr_ref& t).
func (c *bv2RealCall) alignLocals(mark bv2RealCallCheckpoint, s, t *ast.ASTRef) {
	ws, wt := c.width(c.localValue(*s)), c.width(c.localValue(*t))
	if c.err != nil {
		return
	}
	if ws > wt {
		c.assignLocal(mark, t, c.extend(ws-wt, c.localValue(*t)))
	} else if ws < wt {
		c.assignLocal(mark, s, c.extend(wt-ws, c.localValue(*s)))
	}
}
