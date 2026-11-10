package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp bv2real_util::mk_bv_mul,
// mk_bv_add and align_sizes, whose expr_ref locals s1 and t1 release a
// replaced value at the assignment.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
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

// pairLocals is is_bv2real(s, s1, s2, ...) && is_bv2real(t, t1, t2, ...)
// into the caller's expr_ref locals: the matched pieces are owned by the
// locals (a numeral's mk_sbv results are raw in the source).
func (c *bv2RealCall) pairLocals(mark bv2RealCallCheckpoint, s, t ast.ASTHandle) (s1, s2, t1, t2 ast.ASTRef, d1, d2, r *unumeral.OwnedRational, ok bool) {
	a, b, x, y, d1, d2, r, ok := c.pair(s, t)
	if !ok || c.err != nil {
		return s1, s2, t1, t2, d1, d2, r, false
	}
	s1, s2, t1, t2 = c.newLocal(a), c.newLocal(b), c.newLocal(x), c.newLocal(y)
	for _, h := range []ast.ASTHandle{a, b, x, y} {
		c.dropHold(mark.refs, h)
	}
	return s1, s2, t1, t2, d1, d2, r, c.err == nil
}

// alignDivisorsLocals is align_divisors(expr_ref& s1, s2, t1, t2, d1, d2):
// each assignment releases the replaced value.
func (c *bv2RealCall) alignDivisorsLocals(mark bv2RealCallCheckpoint, s1, s2, t1, t2 *ast.ASTRef, d1, d2 *unumeral.OwnedRational) {
	if c.err != nil {
		return
	}
	s := c.u.manager.NumeralService()
	equal, err := s.Equal(d1, d2)
	if err != nil || equal {
		c.err = err
		return
	}
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	g := c.takeRational(s.GCD(d1, d2))
	l := c.takeRational(s.LCM(d1, d2))
	d1g := c.takeRational(s.Div(d1, g))
	d2g := c.takeRational(s.Div(d2, g))
	if c.err != nil {
		return
	}
	c.assignLocal(mark, s1, c.multiplyScalarOwned(d2g, c.localValue(*s1)))
	c.assignLocal(mark, s2, c.multiplyScalarOwned(d2g, c.localValue(*s2)))
	c.assignLocal(mark, t1, c.multiplyScalarOwned(d1g, c.localValue(*t1)))
	c.assignLocal(mark, t2, c.multiplyScalarOwned(d1g, c.localValue(*t2)))
	if c.err != nil {
		return
	}
	if c.err = s.Copy(d1, l); c.err != nil {
		return
	}
	c.err = s.Copy(d2, l)
}
