package opt

// Source: src/util/rational.h. The opt module's weights, offsets and bounds
// are rationals of the AST manager's rational manager
// (ast.ASTManager.NumeralService().Manager), as z3's rational values are
// g_mpq_manager mpqs.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// ratOps applies rational arithmetic with a sticky first error: z3's
// rational operators cannot fail except by throwing (out of memory), so a
// translated expression sequence runs to completion and the caller checks
// err once, as the source's exception would unwind.
type ratOps struct {
	q   *numeral.RationalManager
	err error
}

func newRatOps(m *ast.ASTManager) ratOps {
	if m == nil || m.NumeralService() == nil {
		return ratOps{err: ast.ErrASTHandle}
	}
	return ratOps{q: m.NumeralService().Manager}
}

func (o *ratOps) fail(err error) {
	if err != nil && o.err == nil {
		o.err = err
	}
}

func (o *ratOps) ok() bool { return o.err == nil }

// zero returns a fresh rational 0 (rational()).
func (o *ratOps) zero() numeral.Rational { return numeral.NewRational() }

// int64 returns a fresh rational(v).
func (o *ratOps) int64(v int64) numeral.Rational {
	r := numeral.NewRational()
	if o.ok() {
		o.fail(o.q.SetInt64(&r, v))
	}
	return r
}

// clone returns a fresh copy of src (the rational copy constructor).
func (o *ratOps) clone(src *numeral.Rational) numeral.Rational {
	r := numeral.NewRational()
	o.set(&r, src)
	return r
}

func (o *ratOps) set(dst, src *numeral.Rational) {
	if o.ok() && dst != src {
		o.fail(o.q.Copy(dst, src))
	}
}

func (o *ratOps) setInt64(dst *numeral.Rational, v int64) {
	if o.ok() {
		o.fail(o.q.SetInt64(dst, v))
	}
}

func (o *ratOps) add(dst, a, b *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.Add(a, b, dst))
	}
}

func (o *ratOps) sub(dst, a, b *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.Sub(a, b, dst))
	}
}

func (o *ratOps) mul(dst, a, b *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.Mul(a, b, dst))
	}
}

func (o *ratOps) div(dst, a, b *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.Div(a, b, dst))
	}
}

func (o *ratOps) neg(dst *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.Neg(dst))
	}
}

func (o *ratOps) cmp(a, b *numeral.Rational) int {
	if !o.ok() {
		return 0
	}
	c, err := o.q.Compare(a, b)
	o.fail(err)
	return c
}

func (o *ratOps) sign(a *numeral.Rational) int {
	if !o.ok() {
		return 0
	}
	s, err := o.q.Sign(a)
	o.fail(err)
	return int(s)
}

func (o *ratOps) isInt(a *numeral.Rational) bool {
	if !o.ok() {
		return false
	}
	v, err := o.q.IsInt(a)
	o.fail(err)
	return v
}

func (o *ratOps) isOne(a *numeral.Rational) bool {
	if !o.ok() {
		return false
	}
	v, err := o.q.IsOne(a)
	o.fail(err)
	return v
}

// text is rational::to_string: "n" or "n/d".
func (o *ratOps) text(a *numeral.Rational) string {
	if !o.ok() {
		return ""
	}
	s, err := o.q.String(a)
	o.fail(err)
	return s
}

// del releases r (the rational destructor) and leaves it a valid 0.
func (o *ratOps) del(r *numeral.Rational) {
	if r == nil {
		return
	}
	if o.q != nil {
		o.fail(o.q.Del(r))
	}
	*r = numeral.NewRational()
}

func (o *ratOps) delAll(rs []numeral.Rational) {
	for i := len(rs) - 1; i >= 0; i-- {
		o.del(&rs[i])
	}
}

// joined returns err joined with the sticky error.
func (o *ratOps) joined(err error) error { return errors.Join(err, o.err) }
