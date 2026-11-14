package smtcore

// Allocation-light rational arithmetic for the arithmetic theory's hot
// paths. Each helper computes exactly the value math/big would compute
// (results are always in big.Rat's normal form: gcd(num, den) = 1 and
// den > 0), but zero, small and integer operands skip big.Rat's norm
// (lehmerGCD) and its temporary copies; small operands use machine
// words. Larger fractions use math/big unchanged. Like the math/big
// methods, the destination may alias either operand. These are
// representation details of mpq arithmetic, not project algorithms.

import (
	"math/big"
	"math/bits"
)

// smtRatSmallLimit bounds the magnitudes handled by the word fast path:
// with |a|, |c|, b, d below 2^31 the products a*d, c*b, b*d stay below
// 2^62 and their sum below 2^63.
const smtRatSmallLimit = 1 << 31

// smtRatSmall returns x = n/d when |n| and d are below smtRatSmallLimit.
func smtRatSmall(x *big.Rat) (n int64, d int64, ok bool) {
	num := x.Num()
	if !num.IsInt64() {
		return 0, 0, false
	}
	n = num.Int64()
	if n <= -smtRatSmallLimit || n >= smtRatSmallLimit {
		return 0, 0, false
	}
	if x.IsInt() {
		// IsInt first: Denom allocates for an uninitialized Rat.
		return n, 1, true
	}
	den := x.Denom()
	if !den.IsInt64() {
		return 0, 0, false
	}
	d = den.Int64()
	if d >= smtRatSmallLimit {
		return 0, 0, false
	}
	return n, d, true
}

func smtGCDUint64(a, b uint64) uint64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// smtRatSetNormal sets z = n/d for d > 0, reducing by the gcd.
func smtRatSetNormal(z *big.Rat, n, d int64) *big.Rat {
	if n == 0 {
		return z.SetInt64(0)
	}
	if d != 1 {
		an := n
		if an < 0 {
			an = -an
		}
		if g := int64(smtGCDUint64(uint64(an), uint64(d))); g != 1 {
			n /= g
			d /= g
		}
	}
	// SetInt64 leaves an initialized denominator 1, so Denom is a
	// reference to z's denominator.
	z.SetInt64(n)
	if d != 1 {
		z.Denom().SetInt64(d)
	}
	return z
}

// smtRatSetIntDen makes z's denominator 1 after its numerator was set
// directly to an integer result.
func smtRatSetIntDen(z *big.Rat) {
	if !z.IsInt() {
		z.Denom().SetInt64(1)
	}
}

// smtRatAdd sets z = x + y.
func smtRatAdd(z, x, y *big.Rat) *big.Rat {
	if y.Sign() == 0 {
		if z != x {
			z.Set(x)
		}
		return z
	}
	if x.Sign() == 0 {
		if z != y {
			z.Set(y)
		}
		return z
	}
	if a, b, ok := smtRatSmall(x); ok {
		if c, d, ok := smtRatSmall(y); ok {
			if b == d {
				return smtRatSetNormal(z, a+c, b)
			}
			return smtRatSetNormal(z, a*d+c*b, b*d)
		}
	}
	if x.IsInt() && y.IsInt() {
		z.Num().Add(x.Num(), y.Num())
		smtRatSetIntDen(z)
		return z
	}
	return z.Add(x, y)
}

// smtRatSub sets z = x - y.
func smtRatSub(z, x, y *big.Rat) *big.Rat {
	if y.Sign() == 0 {
		if z != x {
			z.Set(x)
		}
		return z
	}
	if x.Sign() == 0 {
		return z.Neg(y)
	}
	if a, b, ok := smtRatSmall(x); ok {
		if c, d, ok := smtRatSmall(y); ok {
			if b == d {
				return smtRatSetNormal(z, a-c, b)
			}
			return smtRatSetNormal(z, a*d-c*b, b*d)
		}
	}
	if x.IsInt() && y.IsInt() {
		z.Num().Sub(x.Num(), y.Num())
		smtRatSetIntDen(z)
		return z
	}
	return z.Sub(x, y)
}

// smtRatMul sets z = x * y.
func smtRatMul(z, x, y *big.Rat) *big.Rat {
	if x.Sign() == 0 || y.Sign() == 0 {
		return z.SetInt64(0)
	}
	if a, b, ok := smtRatSmall(x); ok {
		if c, d, ok := smtRatSmall(y); ok {
			return smtRatSetNormal(z, a*c, b*d)
		}
	}
	if x.IsInt() && y.IsInt() {
		z.Num().Mul(x.Num(), y.Num())
		smtRatSetIntDen(z)
		return z
	}
	return z.Mul(x, y)
}

// smtRatQuo sets z = x / y; like big.Rat.Quo it panics when y is zero.
func smtRatQuo(z, x, y *big.Rat) *big.Rat {
	if y.Sign() != 0 && x.Sign() == 0 {
		return z.SetInt64(0)
	}
	if a, b, ok := smtRatSmall(x); ok {
		if c, d, ok := smtRatSmall(y); ok && c != 0 {
			n, m := a*d, b*c
			if m < 0 {
				n, m = -n, -m
			}
			return smtRatSetNormal(z, n, m)
		}
	}
	return z.Quo(x, y)
}

// smtIntLCM sets z = lcm(|a|, |b|), or 0 when either is 0; z may alias a
// or b. scratch, which may be nil, must not alias z, a or b.
func smtIntLCM(z, a, b, scratch *big.Int) *big.Int {
	if a.Sign() == 0 || b.Sign() == 0 {
		return z.SetInt64(0)
	}
	if a.IsUint64() && b.IsUint64() {
		x, y := a.Uint64(), b.Uint64()
		q := x / smtGCDUint64(x, y)
		if hi, lo := bits.Mul64(q, y); hi == 0 {
			return z.SetUint64(lo)
		}
	}
	if scratch == nil {
		scratch = new(big.Int)
	}
	scratch.GCD(nil, nil, a, b)
	if z == b {
		// keep b while z is overwritten.
		scratch.Quo(a, scratch)
		z.Mul(scratch, b)
	} else {
		z.Quo(a, scratch)
		z.Mul(z, b)
	}
	return z.Abs(z)
}
