package simplex

// Source: src/util/rational.h, src/util/mpq.h. z3's rational and mpq
// operators cannot fail except by throwing (out of memory), so a translated
// expression sequence runs to completion and the caller checks the first
// error once, as the source's exception would unwind.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// ratCalc applies rational arithmetic on q with a sticky first error.
type ratCalc struct {
	q   *numeral.RationalManager
	err error
}

func (o *ratCalc) fail(err error) {
	if err != nil && o.err == nil {
		o.err = err
	}
}

func (o *ratCalc) ok() bool { return o.err == nil }

// int64 returns a fresh rational(v).
func (o *ratCalc) int64(v int64) numeral.Rational {
	r := numeral.NewRational()
	if v != 0 && o.ok() {
		o.fail(o.q.SetInt64(&r, v))
	}
	return r
}

// clone returns a fresh copy of src (the rational copy constructor).
func (o *ratCalc) clone(src *numeral.Rational) numeral.Rational {
	r := numeral.NewRational()
	o.set(&r, src)
	return r
}

func (o *ratCalc) set(dst, src *numeral.Rational) {
	if o.ok() && dst != src {
		o.fail(o.q.Copy(dst, src))
	}
}

func (o *ratCalc) setInt64(dst *numeral.Rational, v int64) {
	if o.ok() {
		o.fail(o.q.SetInt64(dst, v))
	}
}

func (o *ratCalc) add(dst, a, b *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.Add(a, b, dst))
	}
}

func (o *ratCalc) sub(dst, a, b *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.Sub(a, b, dst))
	}
}

func (o *ratCalc) mul(dst, a, b *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.Mul(a, b, dst))
	}
}

func (o *ratCalc) div(dst, a, b *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.Div(a, b, dst))
	}
}

// addMul is dst = a + b*c.
func (o *ratCalc) addMul(dst, a, b, c *numeral.Rational) {
	if o.ok() {
		if err := o.q.AddMul(a, b, c, dst); err != nil {
			// The source's addmul temporary is a scoped local that the
			// exception unwind destroys; release it so the manager is
			// usable again after the failure.
			o.fail(errors.Join(err, o.q.CleanupFusedTemporary()))
		}
	}
}

func (o *ratCalc) neg(dst *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.Neg(dst))
	}
}

func (o *ratCalc) abs(dst *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.Abs(dst))
	}
}

func (o *ratCalc) inv(dst *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.InvInPlace(dst))
	}
}

func (o *ratCalc) mod(dst, a, b *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.Mod(a, b, dst))
	}
}

func (o *ratCalc) gcd(dst, a, b *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.GCD(a, b, dst))
	}
}

func (o *ratCalc) lcm(dst, a, b *numeral.Rational) {
	if o.ok() {
		o.fail(o.q.LCM(a, b, dst))
	}
}

func (o *ratCalc) cmp(a, b *numeral.Rational) int {
	if !o.ok() {
		return 0
	}
	c, err := o.q.Compare(a, b)
	o.fail(err)
	return c
}

// sign is -1, 0 or 1 (the manager's Sign reports any value of that sign).
func (o *ratCalc) sign(a *numeral.Rational) int {
	if !o.ok() {
		return 0
	}
	s, err := o.q.Sign(a)
	o.fail(err)
	return normalSignOf(s)
}

func normalSignOf(s int32) int {
	switch {
	case s < 0:
		return -1
	case s > 0:
		return 1
	}
	return 0
}

func (o *ratCalc) isInt(a *numeral.Rational) bool {
	if !o.ok() {
		return false
	}
	v, err := o.q.IsInt(a)
	o.fail(err)
	return v
}

func (o *ratCalc) isOne(a *numeral.Rational) bool {
	if !o.ok() {
		return false
	}
	v, err := o.q.IsOne(a)
	o.fail(err)
	return v
}

func (o *ratCalc) isMinusOne(a *numeral.Rational) bool {
	if !o.ok() {
		return false
	}
	v, err := o.q.IsMinusOne(a)
	o.fail(err)
	return v
}

// text is rational::to_string: "n" or "n/d".
func (o *ratCalc) text(a *numeral.Rational) string {
	if !o.ok() {
		return ""
	}
	s, err := o.q.String(a)
	o.fail(err)
	return s
}

// del releases r (the rational destructor) and leaves it a valid 0. A
// destructor runs even after an earlier failure.
func (o *ratCalc) del(r *numeral.Rational) {
	if r == nil {
		return
	}
	o.fail(o.q.Del(r))
	*r = numeral.NewRational()
}

func (o *ratCalc) delAll(rs []numeral.Rational) {
	for i := len(rs) - 1; i >= 0; i-- {
		o.del(&rs[i])
	}
}

// joined returns err joined with the sticky error.
func (o *ratCalc) joined(err error) error { return errors.Join(err, o.err) }

// take returns the sticky error and clears it.
func (o *ratCalc) take() error {
	err := o.err
	o.err = nil
	return err
}
