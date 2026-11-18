package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:159-176.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// rationalFrom copies a rational of the manager's NumeralService().Manager
// into a project owner of this call.
func (c *bv2RealCall) rationalFrom(r *unumeral.Rational) *unumeral.OwnedRational {
	v := c.number()
	if v != nil {
		c.err = c.u.manager.NumeralService().Manager.Copy(&v.Raw, r)
	}
	if c.err != nil {
		return nil
	}
	return v
}

// takeRational accepts ownership of a project arithmetic return, including
// any nonzero owner returned with an error so cleanup never loses it.
func (c *bv2RealCall) takeRational(v unumeral.OwnedRational, err error) *unumeral.OwnedRational {
	if v != (unumeral.OwnedRational{}) {
		c.numbers = append(c.numbers, &v)
	}
	c.err = errors.Join(c.err, err)
	if c.err != nil {
		return nil
	}
	if v == (unumeral.OwnedRational{}) {
		c.err = ast.ErrASTDomain
		return nil
	}
	return &v
}

// alignDivisors is align_divisors(s1, s2, t1, t2, d1, d2): d1 and d2 are
// the caller's rationals, set to lcm(d1, d2).
func (c *bv2RealCall) alignDivisors(a, b, x, y ast.ASTHandle, d1, d2 *unumeral.OwnedRational) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
	if c.err != nil {
		return a, b, x, y
	}
	s := c.u.manager.NumeralService()
	equal, err := s.Equal(d1, d2)
	if err != nil {
		c.err = err
		return a, b, x, y
	}
	if equal {
		return a, b, x, y
	}
	for _, d := range []*unumeral.OwnedRational{d1, d2} {
		integer, err := s.IsInt(d)
		if err != nil {
			c.err = err
			return a, b, x, y
		}
		if !integer {
			c.err = ast.ErrASTDomain
			return a, b, x, y
		}
	}
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	first, second := d1, d2
	g := c.takeRational(s.GCD(first, second))
	if c.err != nil {
		return a, b, x, y
	}
	l := c.takeRational(s.LCM(first, second))
	if c.err != nil {
		return a, b, x, y
	}
	d1g := c.takeRational(s.Div(first, g))
	if c.err != nil {
		return a, b, x, y
	}
	d2g := c.takeRational(s.Div(second, g))
	if c.err != nil {
		return a, b, x, y
	}
	left, right := d2g, d1g
	// Each expr_ref assignment completes before the next scaling operation.
	next := c.multiplyScalar(left, a)
	if c.err != nil {
		return a, b, x, y
	}
	a = next
	next = c.multiplyScalar(left, b)
	if c.err != nil {
		return a, b, x, y
	}
	b = next
	next = c.multiplyScalar(right, x)
	if c.err != nil {
		return a, b, x, y
	}
	x = next
	next = c.multiplyScalar(right, y)
	if c.err != nil {
		return a, b, x, y
	}
	y = next
	// The source copies the common divisor only after all four assignments.
	if c.err = s.Copy(d1, l); c.err != nil {
		return a, b, x, y
	}
	c.err = s.Copy(d2, l)
	return a, b, x, y
}
