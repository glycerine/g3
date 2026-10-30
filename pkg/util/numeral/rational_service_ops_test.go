package numeral

// Source assertions from src/test/rational.cpp; test helpers own every returned token.

import (
	"errors"
	"math"
	"math/big"
	"testing"
)

type facadeHarness struct {
	t     *testing.T
	s     *RationalService
	owned []*OwnedRational
	count int
}

func facadeNew(t *testing.T) *facadeHarness {
	s, _ := serviceNew(t)
	h := &facadeHarness{t: t, s: s}
	t.Cleanup(func() {
		for i := len(h.owned) - 1; i >= 0; i-- {
			serviceMust(t, s.Del(h.owned[i]))
		}
	})
	return h
}
func (h *facadeHarness) keep(v OwnedRational, e error) *OwnedRational {
	h.t.Helper()
	serviceMust(h.t, e)
	p := &v
	h.owned = append(h.owned, p)
	return p
}
func (h *facadeHarness) r(text string) *OwnedRational { return h.keep(h.sValue(text)) }
func (h *facadeHarness) sValue(text string) (OwnedRational, error) {
	v, e := h.s.NewValue()
	if e == nil {
		e = h.s.SetString(&v, text)
	}
	return v, e
}
func (h *facadeHarness) fraction(n, d int32) *OwnedRational {
	v, e := h.s.NewValue()
	if e == nil {
		e = h.s.SetFractionSigned(&v, n, d)
	}
	return h.keep(v, e)
}
func (h *facadeHarness) eq(a, b *OwnedRational) bool {
	v, e := h.s.Equal(a, b)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) ensure(v bool) {
	h.t.Helper()
	h.count++
	if !v {
		h.t.Fatalf("source assertion %d", h.count)
	}
}
func (h *facadeHarness) add(a, b *OwnedRational) *OwnedRational  { return h.keep(h.s.Add(a, b)) }
func (h *facadeHarness) sub(a, b *OwnedRational) *OwnedRational  { return h.keep(h.s.Sub(a, b)) }
func (h *facadeHarness) mul(a, b *OwnedRational) *OwnedRational  { return h.keep(h.s.Mul(a, b)) }
func (h *facadeHarness) div(a, b *OwnedRational) *OwnedRational  { return h.keep(h.s.Div(a, b)) }
func (h *facadeHarness) rem(a, b *OwnedRational) *OwnedRational  { return h.keep(h.s.Rem(a, b)) }
func (h *facadeHarness) idiv(a, b *OwnedRational) *OwnedRational { return h.keep(h.s.IDiv(a, b)) }
func (h *facadeHarness) gcd(a, b *OwnedRational) *OwnedRational  { return h.keep(h.s.GCD(a, b)) }
func (h *facadeHarness) neg(a *OwnedRational) *OwnedRational     { return h.keep(h.s.Neg(a)) }
func (h *facadeHarness) abs(a *OwnedRational) *OwnedRational     { return h.keep(h.s.Abs(a)) }
func (h *facadeHarness) floor(a *OwnedRational) *OwnedRational   { return h.keep(h.s.Floor(a)) }
func (h *facadeHarness) ceil(a *OwnedRational) *OwnedRational    { return h.keep(h.s.Ceil(a)) }
func (h *facadeHarness) power(a *OwnedRational, p uint32) *OwnedRational {
	return h.keep(h.s.Power(a, p))
}
func (h *facadeHarness) lt(a, b *OwnedRational) bool {
	v, e := h.s.Less(a, b)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) gt(a, b *OwnedRational) bool {
	v, e := h.s.Greater(a, b)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) le(a, b *OwnedRational) bool {
	v, e := h.s.LessEqual(a, b)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) ge(a, b *OwnedRational) bool {
	v, e := h.s.GreaterEqual(a, b)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) is_pos(a *OwnedRational) bool {
	v, e := h.s.IsPos(a)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) is_neg(a *OwnedRational) bool {
	v, e := h.s.IsNeg(a)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) is_zero(a *OwnedRational) bool {
	v, e := h.s.IsZero(a)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) is_one(a *OwnedRational) bool {
	v, e := h.s.IsOne(a)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) is_minus_one(a *OwnedRational) bool {
	v, e := h.s.IsMinusOne(a)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) is_nonneg(a *OwnedRational) bool {
	v, e := h.s.IsNonNeg(a)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) is_nonpos(a *OwnedRational) bool {
	v, e := h.s.IsNonPos(a)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) is_int64(a *OwnedRational) bool {
	v, e := h.s.IsInt64(a)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) is_uint64(a *OwnedRational) bool {
	v, e := h.s.IsUint64(a)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) get_int64(a *OwnedRational) int64 {
	v, e := h.s.Int64(a)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) get_uint64(a *OwnedRational) uint64 {
	v, e := h.s.Uint64(a)
	serviceMust(h.t, e)
	return v
}
func (h *facadeHarness) integer(n int32) *OwnedRational {
	v, e := h.s.NewValue()
	if e == nil {
		e = h.s.SetInt64(&v, int64(n))
	}
	return h.keep(v, e)
}
func (h *facadeHarness) clone(a *OwnedRational) *OwnedRational {
	v, e := h.s.NewValue()
	if e == nil {
		e = h.s.Copy(&v, a)
	}
	return h.keep(v, e)
}
func (h *facadeHarness) ne(a, b *OwnedRational) bool {
	v, e := h.s.NotEqual(a, b)
	serviceMust(h.t, e)
	return v
}
func TestRationalFacadeSourceTST1(t *testing.T) {
	h := facadeNew(t)
	r1 := h.integer(1)
	r2 := h.fraction(1, 2)
	r3 := h.fraction(2, 4)
	h.assert(func() bool { return h.eq(r2, r3) })
	h.assert(func() bool { return h.ne(r1, r2) })
	h.assert(func() bool { return h.eq(h.add(r2, r3), r1) })
	h.assert(func() bool { return h.is_pos(r1) })
	h.assert(func() bool { return h.is_neg(h.sub(r2, r1)) })
	h.assert(func() bool { return h.is_zero(h.sub(r2, r3)) })
	h.assert(func() bool { return h.is_zero(h.floor(r2)) })
	h.assert(func() bool { return h.is_one(h.ceil(r2)) })
	h.assert(func() bool { return h.is_minus_one(h.floor(h.neg(r2))) })
	h.assert(func() bool { return h.is_zero(h.ceil(h.neg(r2))) })
	h.assert(func() bool { return h.eq(h.floor(r1), r1) })
	h.assert(func() bool { return h.eq(h.ceil(r1), r1) })
	r4 := h.fraction(3, 5)
	h.assert(func() bool { return h.eq(h.mul(r3, r4), h.fraction(3, 10)) })
	h.assert(func() bool { return h.eq(h.div(r3, r4), h.fraction(5, 6)) })
	r5 := h.fraction(2, 3)
	h.assert(func() bool { return h.eq(h.mul(r4, r5), h.fraction(2, 5)) })
	serviceMust(t, h.s.PrefixDec(r2))
	h.assert(func() bool { return h.eq(r2, h.neg(r3)) })
	serviceMust(t, h.s.NegAssign(r2))
	h.assert(func() bool { return h.eq(r2, r3) })
	serviceMust(t, h.s.PrefixDec(r2))
	h.assign(r2, func() *OwnedRational { return h.abs(r2) })
	h.assert(func() bool { return h.eq(r2, r3) })
	serviceMust(t, h.s.PrefixDec(r2))
	serviceMust(t, h.s.PrefixInc(r2))
	h.assert(func() bool { return h.eq(r2, r3) })
	h.assert(func() bool { return h.eq(r2, h.abs(r2)) })
	h.assert(func() bool { return h.eq(h.mul(r4, h.integer(1)), r4) })
	h.assert(func() bool { return h.is_zero(h.mul(r4, h.integer(0))) })
	h.assert(func() bool { return h.eq(h.mul(r4, h.integer(-1)), h.neg(r4)) })
	h.assert(func() bool { return h.eq(h.mul(h.integer(1), r4), r4) })
	h.assert(func() bool { return h.is_zero(h.mul(h.integer(0), r4)) })
	h.assert(func() bool { return h.eq(h.mul(h.integer(-1), r4), h.neg(r4)) })
	h.assert(func() bool { return h.eq(h.add(r4, h.integer(0)), r4) })
	h.assert(func() bool { return h.is_one(h.ceil(r4)) })
	h.assert(func() bool { return h.eq(h.div(r3, h.neg(r4)), h.fraction(5, -6)) })
	h.assert(func() bool { return h.eq(h.idiv(h.integer(7), h.integer(2)), h.integer(3)) })
	h.assert(func() bool { return h.eq(h.rem(h.integer(7), h.integer(4)), h.integer(3)) })
	h.assert(func() bool { return h.eq(h.idiv(h.integer(7), h.integer(-2)), h.integer(-3)) })
	h.assert(func() bool { return h.eq(h.add(h.integer(3), h.integer(5)), h.integer(8)) })
	h.assert(func() bool { return h.eq(h.add(h.r("13/10"), h.r("7/10")), h.integer(2)) })
	h.assert(func() bool { return h.eq(h.r("100/20"), h.integer(5)) })
	h.assert(func() bool { return h.eq(h.gcd(h.integer(12), h.integer(8)), h.integer(4)) })
	h.assert(func() bool { return h.eq(h.ceil(h.fraction(-3, 2)), h.integer(-1)) })
	h.assert(func() bool { return h.eq(h.floor(h.fraction(-3, 2)), h.integer(-2)) })
	h.assert(func() bool { return h.eq(h.ceil(h.fraction(3, 2)), h.integer(2)) })
	h.assert(func() bool { return h.eq(h.floor(h.fraction(3, 2)), h.integer(1)) })
	h.assert(func() bool { return h.is_pos(h.integer(3)) })
	h.assert(func() bool { return h.is_nonneg(h.integer(0)) })
	h.assert(func() bool { return h.is_pos(h.integer(3)) })
	h.assert(func() bool { return h.is_nonneg(h.integer(3)) })
	h.assert(func() bool { return h.is_nonneg(h.integer(0)) })
	h.assert(func() bool { return !h.is_zero(h.integer(3)) })
	h.assert(func() bool { return !h.is_zero(h.integer(-3)) })
	h.assert(func() bool { return h.is_zero(h.integer(0)) })
	h.assert(func() bool { return h.is_one(h.integer(1)) })
	h.assert(func() bool { return !h.is_one(h.integer(2)) })
	h.assert(func() bool { return h.ge(h.fraction(3, 4), h.fraction(2, 8)) })
	h.assert(func() bool { return h.le(h.fraction(3, 4), h.fraction(7, 8)) })
	h.assert(func() bool { return h.le(h.fraction(3, 4), h.fraction(3, 4)) })
	h.assert(func() bool { return h.ge(h.fraction(3, 4), h.fraction(3, 4)) })
	h.assert(func() bool { return h.gt(h.fraction(3, 4), h.fraction(2, 8)) })
	h.assert(func() bool { return h.lt(h.fraction(3, 4), h.fraction(7, 8)) })
	h.assert(func() bool { return h.eq(h.add(h.integer(3), h.integer(2)), h.integer(5)) })
	h.assert(func() bool { return h.eq(h.sub(h.integer(3), h.integer(2)), h.integer(1)) })
	h.assert(func() bool { return h.eq(h.mul(h.integer(3), h.integer(2)), h.integer(6)) })
	h.assert(func() bool { return h.eq(h.div(h.integer(6), h.integer(2)), h.integer(3)) })
	h.assert(func() bool { return h.eq(h.rem(h.integer(6), h.integer(4)), h.integer(2)) })
	h.assert(func() bool { return h.eq(h.power(h.integer(2), 0), h.integer(1)) })
	h.assert(func() bool { return h.eq(h.power(h.integer(2), 1), h.integer(2)) })
	h.assert(func() bool { return h.eq(h.power(h.integer(2), 3), h.integer(8)) })
	if h.count != 64 {
		t.Fatalf("assertion count %d", h.count)
	}
}
func TestRationalFacadeSourceTST2(t *testing.T) {
	h := facadeNew(t)
	r1 := h.r("10000000000000000000000000000000000")
	r2 := h.r("10000000000000000000000000000000000/3")
	r3 := h.r("20000000000000000000000000000000000/6")
	h.assert(func() bool { return h.eq(r2, r3) })
	h.assert(func() bool { return h.ne(r1, r2) })
	h.assert(func() bool { return h.eq(h.add(h.mul(h.integer(2), r2), r3), r1) })
	h.assert(func() bool { return h.is_pos(r1) })
	h.assert(func() bool { return h.is_neg(h.sub(r2, r1)) })
	h.assert(func() bool { return h.is_zero(h.sub(r2, r3)) })
	r0 := h.r("1/3000000000000000000000000")
	h.assert(func() bool { return h.is_one(h.ceil(r0)) })
	h.assert(func() bool { return h.is_minus_one(h.floor(h.neg(r0))) })
	h.assert(func() bool { return h.is_zero(h.ceil(h.neg(r0))) })
	h.destroy(r0) // End of the source brace-local r0 lifetime.
	h.assert(func() bool { return h.eq(h.floor(r1), r1) })
	h.assert(func() bool { return h.eq(h.ceil(r1), r1) })
	r4 := h.r("300000000/5")
	h.assert(func() bool { return h.eq(h.mul(h.fraction(1, 2), r4), h.r("300000000/10")) })
	h.assert(func() bool { return h.eq(h.div(h.fraction(1, 2), r4), h.r("5/600000000")) })
	r5 := h.fraction(2, 3)
	h.assert(func() bool { return h.eq(h.mul(r4, r5), h.r("200000000/5")) })
	r6 := h.r("10000000000000000000000000000000003/3")
	serviceMust(t, h.s.PrefixDec(r6))
	h.assert(func() bool { return h.eq(r6, r2) })
	serviceMust(t, h.s.NegAssign(r6))
	h.assert(func() bool { return h.ne(r6, r2) })
	h.assert(func() bool { return h.eq(h.abs(r6), r2) })
	serviceMust(t, h.s.PrefixDec(r2))
	serviceMust(t, h.s.PrefixInc(r2))
	serviceMust(t, h.s.NegAssign(r2))
	h.assert(func() bool { return h.eq(r2, r6) })
	h.assert(func() bool { return h.eq(h.mul(r6, h.integer(1)), r6) })
	h.assert(func() bool { return h.is_zero(h.mul(r6, h.integer(0))) })
	h.assert(func() bool { return h.eq(h.mul(r6, h.integer(-1)), h.neg(r6)) })
	h.assert(func() bool { return h.eq(h.mul(h.integer(1), r6), r6) })
	h.assert(func() bool { return h.is_zero(h.mul(h.integer(0), r6)) })
	h.assert(func() bool { return h.eq(h.mul(h.integer(-1), r6), h.neg(r6)) })
	h.assert(func() bool { return h.eq(h.add(r6, h.integer(0)), r6) })
	h.assert(func() bool { return h.is_pos(h.r("300000000000000")) })
	h.assert(func() bool { return h.is_nonneg(h.r("0000000000000000000")) })
	h.assert(func() bool { return h.is_nonpos(h.r("0000000000000000000")) })
	h.assert(func() bool { return h.is_pos(h.r("3000000000000000000/2")) })
	h.assert(func() bool { return h.is_nonneg(h.r("3000000000000000000/2")) })
	h.assert(func() bool { return h.is_neg(h.neg(h.r("3000000000000000000/2"))) })
	h.assert(func() bool { return !h.is_neg(h.r("3000000000000000000/2")) })
	h.assert(func() bool { return !h.is_zero(h.r("3000000000000000000/2")) })
	h.assert(func() bool { return !h.is_one(h.r("3000000000000000000/2")) })
	h.assert(func() bool { return h.ge(h.r("99999999999/2"), h.r("23/2")) })
	h.assert(func() bool { return h.gt(h.r("99999999999/2"), h.r("23/2")) })
	h.assert(func() bool { return h.le(h.r("23/2"), h.r("99999999999/2")) })
	h.assert(func() bool { return h.lt(h.r("23/2"), h.r("99999999999/2")) })
	h.assert(func() bool { return !h.lt(h.r("99999999999/2"), h.r("23/2")) })
	int64_max := h.r("9223372036854775807")
	int64_min := h.named(func() *OwnedRational { return h.clone(h.sub(h.neg(int64_max), h.integer(1))) })
	h.assert(func() bool { return h.is_int64(int64_max) })
	h.assert(func() bool { return h.is_int64(int64_min) })
	h.assert(func() bool { return h.is_int64(h.integer(0)) })
	h.assert(func() bool { return h.is_int64(h.integer(1)) })
	h.assert(func() bool { return h.is_int64(h.integer(-1)) })
	h.assert(func() bool { return !h.is_int64(h.add(int64_max, h.integer(1))) })
	h.assert(func() bool { return !h.is_int64(h.sub(int64_min, h.integer(1))) })
	h.assert(func() bool { return h.is_uint64(int64_max) })
	h.assert(func() bool { return !h.is_uint64(int64_min) })
	h.assert(func() bool { return h.is_uint64(h.integer(0)) })
	h.assert(func() bool { return h.is_uint64(h.integer(1)) })
	h.assert(func() bool { return !h.is_uint64(h.integer(-1)) })
	h.assert(func() bool { return h.is_uint64(h.add(int64_max, h.integer(1))) })
	h.assert(func() bool { return !h.is_uint64(h.sub(int64_min, h.integer(1))) })
	uint64_max := h.named(func() *OwnedRational { return h.clone(h.add(h.integer(1), h.mul(h.integer(2), int64_max))) })
	h.assert(func() bool { return h.is_uint64(uint64_max) })
	u1 := h.get_uint64(uint64_max)
	u2 := uint64(math.MaxUint64)
	h.assert(func() bool { return (u1 == u2) })
	h.assert(func() bool { return (h.get_int64(int64_max) == int64(math.MaxInt64)) })
	h.assert(func() bool { return (h.get_int64(int64_min) == int64(math.MinInt64)) })
	if h.count != 57 {
		t.Fatalf("assertion count %d", h.count)
	}
}

func TestRationalFacadeSourceTST3AndTST5(t *testing.T) {
	t.Run("large-diagnostic-division", func(t *testing.T) {
		h := facadeNew(t)
		n1 := h.r("4294967293")
		n2 := h.idiv(n1, h.integer(2))
		h.assert(func() bool { return h.eq(n2, h.integer(2147483646)) })
	})
	t.Run("integer-division-chain", func(t *testing.T) {
		h := facadeNew(t)
		two := h.integer(2)
		n1 := h.power(two, 32)
		n2 := h.idiv(n1, two)
		n3 := h.idiv(two, n2)
		n4 := h.sub(n1, h.integer(3))
		n5 := h.idiv(n4, two)
		h.assert(func() bool { return h.eq(n5, h.integer(2147483646)) })
		_ = n3 // The source computes this diagnostic-only value without asserting it.
	})
	t.Run("large-denominator-in-place-round-trip", func(t *testing.T) {
		h := facadeNew(t)
		n1 := h.integer(-1)
		n2 := h.r("4294967295")
		serviceMust(t, h.s.DivAssign(n1, n2))
		serviceMust(t, h.s.MulAssign(n1, n2))
		h.assert(func() bool { return h.is_minus_one(n1) })
	})
}

func TestRationalFacadeSourceTesterTST1(t *testing.T) {
	h := facadeNew(t)
	n1, n2 := h.integer(-1), h.integer(8)
	h.assert(func() bool { return h.is_minus_one(h.rem(n1, n2)) })
	mod, err := h.s.Mod(n1, n2)
	modValue := h.keep(mod, err)
	h.assert(func() bool { return h.eq(modValue, h.integer(7)) })
	if h.count != 2 {
		t.Fatalf("source assertion count %d", h.count)
	}
}

// Exact arithmetic is an independent bounded oracle; it is never used by the
// production facade. Source-shape fixtures are kept separate below.
func TestRationalFacadeIndependentArithmeticAndAliases(t *testing.T) {
	s, _ := serviceNew(t)
	a, _ := s.NewValue()
	b, _ := s.NewValue()
	defer s.Del(&a)
	defer s.Del(&b)
	valueOps := []func(*OwnedRational, *OwnedRational) (OwnedRational, error){s.Add, s.Sub, s.Mul, s.Div}
	assignOps := []func(*OwnedRational, *OwnedRational) error{s.AddAssign, s.SubAssign, s.MulAssign, s.DivAssign}
	for i := int64(-8); i <= 8; i++ {
		for j := int64(-4); j <= 4; j++ {
			serviceMust(t, s.SetFraction(&a, i, 7))
			serviceMust(t, s.SetFraction(&b, j, 5))
			x, y := new(big.Rat).SetFrac(big.NewInt(i), big.NewInt(7)), new(big.Rat).SetFrac(big.NewInt(j), big.NewInt(5))
			for op, f := range valueOps {
				if op == 3 && j == 0 {
					continue
				}
				want := new(big.Rat)
				switch op {
				case 0:
					want.Add(x, y)
				case 1:
					want.Sub(x, y)
				case 2:
					want.Mul(x, y)
				case 3:
					want.Quo(x, y)
				}
				result, e := f(&a, &b)
				serviceMust(t, e)
				serviceText(t, s, &result, want.RatString())
				serviceMust(t, s.Del(&result))
				left, _ := s.NewValue()
				serviceMust(t, s.Copy(&left, &a))
				serviceMust(t, assignOps[op](&left, &b))
				serviceText(t, s, &left, want.RatString())
				serviceMust(t, s.Del(&left))
			}
			lt, e := s.Less(&a, &b)
			serviceMust(t, e)
			ge, e := s.GreaterEqual(&a, &b)
			serviceMust(t, e)
			if lt != (x.Cmp(y) < 0) || ge != (x.Cmp(y) >= 0) {
				t.Fatal(i, j, lt, ge)
			}
		}
	}
	for op, f := range assignOps {
		serviceMust(t, s.SetFraction(&a, 3, 7))
		serviceMust(t, f(&a, &a))
		serviceText(t, s, &a, []string{"6/7", "0", "9/49", "1"}[op])
	}
}

func TestRationalFacadeScalarOverloadsPostfixAndExtraction(t *testing.T) {
	s, _ := serviceNew(t)
	a := serviceValue(t, s, "3/7")
	defer s.Del(&a)
	right := []func(*OwnedRational, int32) (OwnedRational, error){s.AddInt, s.SubInt, s.MulInt, s.DivInt}
	left := []func(int32, *OwnedRational) (OwnedRational, error){s.IntAdd, s.IntSub, s.IntMul, s.IntDiv}
	for i := range right {
		v, e := right[i](&a, 2)
		serviceMust(t, e)
		serviceText(t, s, &v, []string{"17/7", "-11/7", "6/7", "3/14"}[i])
		serviceMust(t, s.Del(&v))
		v, e = left[i](2, &a)
		serviceMust(t, e)
		serviceText(t, s, &v, []string{"17/7", "11/7", "6/7", "14/3"}[i])
		serviceMust(t, s.Del(&v))
	}
	old, e := s.PostInc(&a)
	serviceMust(t, e)
	serviceText(t, s, &old, "3/7")
	serviceText(t, s, &a, "10/7")
	serviceMust(t, s.Del(&old))
	old, e = s.PostDec(&a)
	serviceMust(t, e)
	serviceText(t, s, &old, "10/7")
	serviceText(t, s, &a, "3/7")
	serviceMust(t, s.Del(&old))
	n, e := s.Numerator(&a)
	serviceMust(t, e)
	d, e := s.Denominator(&a)
	serviceMust(t, e)
	serviceText(t, s, &n, "3")
	serviceText(t, s, &d, "7")
	serviceMust(t, s.Del(&n))
	serviceMust(t, s.Del(&d))
	for i, check := range []func(*OwnedRational, int32) (bool, error){s.EqualInt, s.NotEqualInt, s.LessInt, s.GreaterInt} {
		v, e := check(&a, 0)
		serviceMust(t, e)
		if v != []bool{false, true, false, true}[i] {
			t.Fatal(i, v)
		}
	}
	serviceMust(t, s.AddAssignInt(&a, 2))
	serviceText(t, s, &a, "17/7")
	serviceMust(t, s.AssignInt(&a, -19))
	serviceText(t, s, &a, "-19")
	serviceMust(t, s.Reset(&a))
	serviceText(t, s, &a, "0")
}

func TestRationalFacadeFusedSourceOperandRoutes(t *testing.T) {
	s, _ := serviceNew(t)
	for subtract := 0; subtract < 2; subtract++ {
		for c := -2; c <= 2; c++ {
			for alias := 0; alias < 4; alias++ {
				a := serviceValue(t, s, "3/7")
				co := serviceValue(t, s, big.NewInt(int64(c)).String())
				k := serviceValue(t, s, "5/11")
				cp, kp := &co, &k
				if alias == 1 {
					cp = &a
				}
				if alias == 2 {
					kp = &a
				}
				if alias == 3 {
					cp, kp = &a, &a
				}
				ax, _ := new(big.Rat).SetString("3/7")
				cx, _ := new(big.Rat).SetString(big.NewInt(int64(c)).String())
				kx, _ := new(big.Rat).SetString("5/11")
				if alias == 1 || alias == 3 {
					cx.Set(ax)
				}
				if alias == 2 || alias == 3 {
					kx.Set(ax)
				}
				want := new(big.Rat).Mul(cx, kx)
				if subtract == 1 {
					want.Sub(ax, want)
				} else {
					want.Add(ax, want)
				}
				var e error
				if subtract == 1 {
					e = s.SubMulAssign(&a, cp, kp)
				} else {
					e = s.AddMulAssign(&a, cp, kp)
				}
				serviceMust(t, e)
				serviceText(t, s, &a, want.RatString())
				serviceMust(t, s.Del(&a))
				serviceMust(t, s.Del(&co))
				serviceMust(t, s.Del(&k))
			}
		}
	}
}

func TestRationalFacadeModHatNumBitsAndPowerDomains(t *testing.T) {
	if modularSourceDebug {
		bindMPNDebugActions(t, "TTT")
	}
	s, _ := serviceNew(t)
	a, _ := s.NewValue()
	b, _ := s.NewValue()
	defer s.Del(&a)
	defer s.Del(&b)
	for d := int64(1); d <= 13; d++ {
		for n := int64(-21); n <= 21; n++ {
			serviceMust(t, s.SetInt64(&a, n))
			serviceMust(t, s.SetInt64(&b, d))
			v, e := s.ModHat(&a, &b)
			serviceMust(t, e)
			want := n % d
			if want < 0 {
				want += d
			}
			if 2*want > d {
				want -= d
			}
			serviceText(t, s, &v, big.NewInt(want).String())
			serviceMust(t, s.Del(&v))
		}
	}
	for n := uint64(0); n < 256; n++ {
		serviceMust(t, s.SetUint64(&a, n))
		v, e := s.NumBits(&a)
		serviceMust(t, e)
		want := uint32(new(big.Int).SetUint64(n).BitLen())
		if want == 0 {
			want = 1
		}
		if v != want {
			t.Fatal(n, v, want)
		}
	}
	serviceMust(t, s.SetInt64(&a, -1))
	if v, e := s.NumBits(&a); modularSourceDebug {
		if !isMPNVerificationThrow(e) {
			t.Fatal(e)
		}
	} else if e != nil || v != 1 {
		t.Fatal("release negative source quotient", v, e)
	}
	serviceMust(t, s.SetFraction(&a, 1, 2))
	if v, e := s.NumBits(&a); modularSourceDebug {
		if !isMPNVerificationThrow(e) {
			t.Fatal(e)
		}
	} else if e != nil || v != 1 {
		t.Fatal("release fractional source quotient", v, e)
	}
	serviceMust(t, s.SetInt64(&b, 0))
	if _, e := s.ModHat(&a, &b); modularSourceDebug {
		if !isMPNVerificationThrow(e) {
			t.Fatal(e)
		}
	} else if !errors.Is(e, ErrDivideByZero) {
		t.Fatal("release reaches source mod divisor", e)
	}
	if v, e := s.Expt(&a, -1); e == nil || v.ownerID != 0 {
		t.Fatal(v, e)
	}
	facadeAudit(t, s, &a, &b)
	serviceMust(t, s.Manager.CleanupRawArithmetic())
	serviceMust(t, s.SetInt64(&a, 0))
	v, e := s.Power(&a, 0)
	serviceMust(t, e)
	serviceText(t, s, &v, "1")
	serviceMust(t, s.Del(&v))
}

func facadeAudit(t *testing.T, s *RationalService, values ...*OwnedRational) {
	t.Helper()
	roots := []*Int{&s.ZeroField.Num, &s.ZeroField.Den, &s.OneField.Num, &s.OneField.Den, &s.MinusOneField.Num, &s.MinusOneField.Den}
	for i := range s.Cache {
		roots = append(roots, &s.Cache[i].Num, &s.Cache[i].Den)
	}
	// Explicit invocation contexts own their transferred lexical prefixes.
	// Enumerate these issued owners before checking the descriptor registry.
	for c := s.lexicalInvocations; c != nil; c = c.next {
		for i := uint8(0); i < c.scope.Count; i++ {
			if c.scope.Done&(1<<i) == 0 {
				roots = append(roots, &c.scope.Roots[i].Num, &c.scope.Roots[i].Den)
			}
		}
	}
	for _, v := range values {
		if v != nil && v.ownerID != 0 {
			roots = append(roots, &v.Raw.Num, &v.Raw.Den)
		}
	}
	qopsAuditLiveOwnership(t, s.Manager, roots)
}

func TestRationalFacadeAllocationFailureOwnership(t *testing.T) {
	// Each prefix is checked before formatting or destruction. Retained indexed
	// integer scratch is an allowed known root; lexical facade temps are not.
	for op := 0; op < 20; op++ {
		failures, success := 0, false
		for fault := int64(0); fault < 1024; fault++ {
			s, budget := servicePublicationDirectOwner(t)
			a := serviceValue(t, s, "123456789012345678901234567890123456789/11")
			b := serviceValue(t, s, "98765432109876543210987654321/13")
			c := serviceValue(t, s, "314159265358979323846264338327/17")
			if op >= 4 && op <= 8 || op == 17 || op == 18 {
				serviceMust(t, s.SetString(&a, "123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"))
				serviceMust(t, s.SetString(&b, "98765432109876543210987654321"))
			}
			before, e := s.String(&a)
			serviceMust(t, e)
			bBefore, e := s.String(&b)
			serviceMust(t, e)
			budget.FailAfter(fault)
			var out OwnedRational
			switch op {
			case 0:
				out, e = s.Add(&a, &b)
			case 1:
				out, e = s.Sub(&a, &b)
			case 2:
				out, e = s.Mul(&a, &b)
			case 3:
				out, e = s.Div(&a, &b)
			case 4:
				out, e = s.Rem(&a, &b)
			case 5:
				out, e = s.IDiv(&a, &b)
			case 6:
				out, e = s.MachineDiv(&a, &b)
			case 7:
				out, e = s.Mod(&a, &b)
			case 8:
				out, e = s.GCD(&a, &b)
			case 9:
				out, e = s.Neg(&a)
			case 10:
				out, e = s.Abs(&a)
			case 11:
				out, e = s.Floor(&a)
			case 12:
				out, e = s.Ceil(&a)
			case 13:
				out, e = s.Power(&a, 3)
			case 14:
				out, e = s.PostInc(&a)
			case 15:
				e = s.AddMulAssign(&a, &c, &b)
			case 16:
				e = s.SubMulAssign(&a, &c, &b)
			case 17:
				out, e = s.ModHat(&a, &b)
			case 18:
				_, e = s.NumBits(&a)
			case 19:
				out, e = s.Numerator(&a)
			}
			facadeAudit(t, s, &a, &b, &c, &out) // immediately, before rendering or Del/Close
			budget.FailAfter(-1)
			if e != nil {
				failures++
				if !numericPhysicalReservationRefusal(e) || out.ownerID != 0 {
					t.Fatal(op, fault, out, e)
				}
				if op == 14 || op == 15 || op == 16 {
					left, _ := new(big.Rat).SetString(before)
					var pn, pd *big.Int
					if op == 14 {
						pn = new(big.Int).Add(left.Num(), left.Denom())
						pd = new(big.Int).Set(left.Denom())
					} else {
						right, _ := new(big.Rat).SetString(bBefore)
						third, _ := new(big.Rat).SetString("314159265358979323846264338327/17")
						tmp := new(big.Rat).Mul(right, third)
						pn = new(big.Int).Mul(left.Num(), tmp.Denom())
						cross := new(big.Int).Mul(tmp.Num(), left.Denom())
						if op == 15 {
							pn.Add(pn, cross)
						} else {
							pn.Sub(pn, cross)
						}
						pd = new(big.Int).Mul(left.Denom(), tmp.Denom())
					}
					addPublicationExpectRaw(t, s.Manager, &a.Raw, left.Num().String(), left.Denom().String(), pn, pd)
				} else {
					serviceIndependentText(t, s, &a, before)
				}
			} else {
				success = true
			}
			serviceIndependentText(t, s, &b, bBefore)
			if out.ownerID != 0 {
				serviceMust(t, s.Del(&out))
			}
			serviceMust(t, s.Del(&a))
			serviceMust(t, s.Del(&b))
			serviceMust(t, s.Del(&c))
			serviceMust(t, s.Close())
			if budget.Used() != 0 {
				t.Fatal(op, fault, budget.Used())
			}
			if success {
				break
			}
		}
		if failures == 0 || !success {
			t.Fatal("incomplete failure sweep", op, failures, success)
		}
	}
}

func TestRationalFacadeProvenanceAndNoncanonicalSourceShapes(t *testing.T) {
	s, budget := serviceNew(t)
	other, _ := serviceNew(t)
	a := serviceValue(t, s, "3")
	foreign := serviceValue(t, other, "2")
	defer s.Del(&a)
	defer other.Del(&foreign)
	used := budget.Used()
	budget.FailAfter(0)
	for _, e := range []error{s.AddAssign(&a, &foreign), s.AddMulAssign(&a, &a, &foreign), s.IDivInto(&a, &foreign, &a)} {
		if !errors.Is(e, ErrRationalServiceOwner) {
			t.Fatal(e)
		}
	}
	for _, f := range []func(*OwnedRational, *OwnedRational) (OwnedRational, error){s.Add, s.Sub, s.Mul, s.Div, s.ModHat} {
		v, e := f(&a, &foreign)
		if !errors.Is(e, ErrRationalServiceOwner) || v.ownerID != 0 {
			t.Fatal(e)
		}
	}
	if _, e := s.Less(&a, &foreign); !errors.Is(e, ErrRationalServiceOwner) {
		t.Fatal(e)
	}
	if _, e := s.NumBits(&foreign); !errors.Is(e, ErrRationalServiceOwner) {
		t.Fatal(e)
	}
	budget.FailAfter(-1)
	if budget.Used() != used {
		t.Fatal("foreign check performed allocation")
	}
	// Raw representations reachable from manager aliases/rat_div remain visible.
	raw := serviceValue(t, s, "1/2")
	defer s.Del(&raw)
	serviceMust(t, s.Manager.Z.SetInt64(&raw.Raw.Num, 2))
	serviceMust(t, s.Manager.Z.SetInt64(&raw.Raw.Den, 4))
	canonical := serviceValue(t, s, "1/2")
	defer s.Del(&canonical)
	eq, e := s.Equal(&raw, &canonical)
	serviceMust(t, e)
	less, e := s.Less(&raw, &canonical)
	serviceMust(t, e)
	if eq || less {
		t.Fatal("structural equality replaced by comparison", eq, less)
	}
	serviceMust(t, s.Manager.Z.SetInt64(&raw.Raw.Num, 1))
	serviceMust(t, s.Manager.Z.SetInt64(&raw.Raw.Den, -3))
	pos, e := s.IsPos(&raw)
	serviceMust(t, e)
	if !pos {
		t.Fatal("source numerator sign shortcut lost")
	}
	v, e := s.MulInt(&raw, 1)
	serviceMust(t, e)
	serviceText(t, s, &v, "1/-3")
	serviceMust(t, s.Del(&v))
}

func TestRationalFacadeWarmZeroAllocations(t *testing.T) {
	s, budget := serviceNew(t)
	a := serviceValue(t, s, "123456789012345678901234567890/7")
	b := serviceValue(t, s, "11/13")
	defer s.Del(&a)
	defer s.Del(&b)
	run := func() {
		v, e := s.Add(&a, &b)
		if e != nil {
			panic(e)
		}
		if e = s.Del(&v); e != nil {
			panic(e)
		}
		v, e = s.Mul(&a, &b)
		if e != nil {
			panic(e)
		}
		if e = s.Del(&v); e != nil {
			panic(e)
		}
		v, e = s.Abs(&a)
		if e != nil {
			panic(e)
		}
		if e = s.Del(&v); e != nil {
			panic(e)
		}
	}
	for i := 0; i < 5; i++ {
		run()
	}
	beforeWarm := numericWarmProfileSnapshot(t, s.Manager.Z, budget)
	n := testing.AllocsPerRun(50, run)
	numericWarmProfileAudit(t, s.Manager.Z, budget, beforeWarm, n, s.Manager.Z.Allocator.Stats().DirectObjects > 0)
}

func TestRationalFacadeInspectionForwardingAndSourceOutputAliases(t *testing.T) {
	s, _ := serviceNew(t)
	a := serviceValue(t, s, "4294967295")
	b := serviceValue(t, s, "7")
	defer s.Del(&a)
	defer s.Del(&b)
	u, e := s.Unsigned(&a)
	serviceMust(t, e)
	if u != math.MaxUint32 {
		t.Fatal(u)
	}
	ok, e := s.IsUnsigned(&a)
	serviceMust(t, e)
	if !ok {
		t.Fatal("inclusive source uint32 bound")
	}
	serviceMust(t, s.SetInt64(&a, math.MinInt32))
	i, e := s.Int32(&a)
	serviceMust(t, e)
	ok, e = s.IsInt32(&a)
	serviceMust(t, e)
	if i != math.MinInt32 || !ok {
		t.Fatal(i, ok)
	}
	serviceMust(t, s.SetString(&a, "3/4"))
	for _, query := range []func(*OwnedRational) (bool, error){s.IsInt, s.IsSmall, s.IsBig, s.IsEven, s.IsUnsigned, s.IsInt32, s.IsUint64, s.IsInt64} {
		_, e := query(&a)
		serviceMust(t, e)
	}
	bits, e := s.Bitsize(&a)
	serviceMust(t, e)
	storage, e := s.StorageSize(&a)
	serviceMust(t, e)
	hash, e := s.Hash(&a)
	serviceMust(t, e)
	rawBits, e := s.Manager.Bitsize(&a.Raw)
	serviceMust(t, e)
	rawStorage, e := s.Manager.StorageSize(&a.Raw)
	serviceMust(t, e)
	rawHash, e := s.Manager.Hash(&a.Raw)
	serviceMust(t, e)
	f, e := s.Float64(&a)
	serviceMust(t, e)
	if bits != rawBits || storage != rawStorage || hash != rawHash || f != 0.75 {
		t.Fatal(bits, storage, hash, f)
	}
	serviceMust(t, s.SetInt64(&a, 64))
	ok, shift, e := s.IsPowerOfTwo(&a)
	serviceMust(t, e)
	if !ok || shift != 6 {
		t.Fatal(ok, shift)
	}
	for _, divisor := range []int64{7, -7} {
		serviceMust(t, s.SetInt64(&a, -26))
		serviceMust(t, s.SetInt64(&b, divisor))
		v, e := s.IDiv(&a, &b)
		serviceMust(t, e)
		want := "-4"
		if divisor < 0 {
			want = "4"
		}
		serviceText(t, s, &v, want)
		serviceMust(t, s.Del(&v))
		serviceMust(t, s.IDivInto(&a, &b, &b))
		want = "-2"
		if divisor < 0 {
			want = "2"
		}
		serviceText(t, s, &b, want)
		serviceMust(t, s.SetInt64(&b, divisor))
		serviceMust(t, s.ModInto(&a, &b, &b))
		serviceText(t, s, &b, "0")
	}
}

func TestRationalFacadeCompoundFailureAliases(t *testing.T) {
	for op := 0; op < 9; op++ {
		for alias := 0; alias < 2; alias++ {
			failures, success := 0, false
			for fault := int64(0); fault < 1024; fault++ {
				s, budget := serviceNew(t)
				a := serviceValue(t, s, "123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890/11")
				b := serviceValue(t, s, "98765432109876543210987654321/13")
				if op == 4 || op == 7 || op == 8 {
					serviceMust(t, s.SetString(&a, "123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"))
					serviceMust(t, s.SetString(&b, "98765432109876543210987654321"))
				}
				if alias == 1 && op < 5 {
					serviceMust(t, s.Copy(&b, &a))
				}
				before, e := s.String(&a)
				serviceMust(t, e)
				bBefore, e := s.String(&b)
				serviceMust(t, e)
				rhs := &b
				if alias == 1 && op < 5 {
					rhs = &a
				}
				budget.FailAfter(fault)
				switch op {
				case 0:
					e = s.AddAssign(&a, rhs)
				case 1:
					e = s.SubAssign(&a, rhs)
				case 2:
					e = s.MulAssign(&a, rhs)
				case 3:
					e = s.DivAssign(&a, rhs)
				case 4:
					e = s.RemAssign(&a, rhs)
				case 5:
					e = s.AddMulAssign(&a, &a, &b)
				case 6:
					e = s.SubMulAssign(&a, &b, &a)
				case 7:
					if alias == 0 {
						e = s.IDivInto(&a, &b, &a)
					} else {
						e = s.IDivInto(&a, &b, &b)
					}
				case 8:
					if alias == 0 {
						e = s.ModInto(&a, &b, &a)
					} else {
						e = s.ModInto(&a, &b, &b)
					}
				}
				facadeAudit(t, s, &a, &b)
				budget.FailAfter(-1)
				if e != nil {
					failures++
					if !numericPhysicalReservationRefusal(e) {
						t.Fatal(op, alias, fault, e)
					}
					if op == 2 {
						left, ok := new(big.Rat).SetString(before)
						if !ok {
							t.Fatal(before)
						}
						right, ok := new(big.Rat).SetString(bBefore)
						if !ok {
							t.Fatal(bBefore)
						}
						if alias == 1 {
							right.Set(left)
						}
						product := new(big.Rat).Mul(left, right)
						rawNum := new(big.Int).Mul(left.Num(), right.Num()).String()
						rawDen := new(big.Int).Mul(left.Denom(), right.Denom()).String()
						num := powerPublicationComponent(t, s.Manager.Z, &a.Raw.Num, left.Num().String(), rawNum, product.Num().String())
						powerPublicationComponent(t, s.Manager.Z, &a.Raw.Den, left.Denom().String(), rawDen, product.Denom().String())
						if num == "failed" || num == left.Num().String() {
							if roundPublicationDecode(t, s.Manager.Z, &a.Raw.Den).String() != left.Denom().String() {
								t.Fatal("denominator preceded numerator MulAssign", alias, fault)
							}
						}
					} else if op == 3 {
						left, ok := new(big.Rat).SetString(before)
						if !ok {
							t.Fatal(before)
						}
						right, ok := new(big.Rat).SetString(bBefore)
						if !ok {
							t.Fatal(bBefore)
						}
						if alias == 1 {
							right.Set(left)
						}
						quotient := new(big.Rat).Quo(left, right)
						rawNum := new(big.Int).Mul(left.Num(), right.Denom())
						rawDen := new(big.Int).Mul(left.Denom(), right.Num())
						numCandidates := []string{left.Num().String(), rawNum.String(), quotient.Num().String()}
						denCandidates := []string{left.Denom().String(), rawDen.String(), quotient.Denom().String()}
						if rawDen.Sign() < 0 {
							numCandidates = append(numCandidates, new(big.Int).Neg(rawNum).String())
							denCandidates = append(denCandidates, new(big.Int).Neg(rawDen).String())
						}
						powerPublicationComponent(t, s.Manager.Z, &a.Raw.Num, numCandidates...)
						powerPublicationComponent(t, s.Manager.Z, &a.Raw.Den, denCandidates...)
					} else if op == 0 || op == 1 || op == 5 || op == 6 {
						left, _ := new(big.Rat).SetString(before)
						right, _ := new(big.Rat).SetString(bBefore)
						if alias == 1 && op < 5 {
							right.Set(left)
						}
						if op == 5 || op == 6 {
							right.Mul(left, right)
						}
						pn := new(big.Int).Mul(left.Num(), right.Denom())
						cross := new(big.Int).Mul(right.Num(), left.Denom())
						if op == 0 || op == 5 {
							pn.Add(pn, cross)
						} else {
							pn.Sub(pn, cross)
						}
						pd := new(big.Int).Mul(left.Denom(), right.Denom())
						addPublicationExpectRaw(t, s.Manager, &a.Raw, left.Num().String(), left.Denom().String(), pn, pd)
					} else {
						serviceIndependentText(t, s, &a, before)
					}
					serviceIndependentText(t, s, &b, bBefore)
				} else {
					success = true
				}
				serviceMust(t, s.Del(&a))
				serviceMust(t, s.Del(&b))
				serviceMust(t, s.Close())
				if budget.Used() != 0 {
					t.Fatal(op, alias, fault, budget.Used())
				}
				if success {
					break
				}
			}
			// Source small/zero fast paths (self subtraction/division/remainder) may need
			// no reservation in an already prepared owner; require completion, not a
			// fabricated failure at a nonexistent allocation point.
			if !success {
				t.Fatal("sweep never completes", op, alias, failures)
			}
		}
	}
}

// Assertion temporaries die at the source full-expression boundary. Named
// caller values remain known owning roots until their lexical scope ends.
func (h *facadeHarness) cleanFrom(mark int) {
	for i := len(h.owned) - 1; i >= mark; i-- {
		serviceMust(h.t, h.s.Del(h.owned[i]))
	}
	h.owned = h.owned[:mark]
}
func (h *facadeHarness) assert(condition func() bool) {
	h.t.Helper()
	mark := len(h.owned)
	defer h.cleanFrom(mark)
	h.ensure(condition())
}
func (h *facadeHarness) named(expression func() *OwnedRational) *OwnedRational {
	mark := len(h.owned)
	result := expression()
	for i := len(h.owned) - 1; i >= mark; i-- {
		if h.owned[i] != result {
			serviceMust(h.t, h.s.Del(h.owned[i]))
		}
	}
	h.owned = h.owned[:mark]
	h.owned = append(h.owned, result)
	return result
}
func (h *facadeHarness) assign(dst *OwnedRational, expression func() *OwnedRational) {
	mark := len(h.owned)
	defer h.cleanFrom(mark)
	serviceMust(h.t, h.s.Copy(dst, expression()))
}
func (h *facadeHarness) destroy(value *OwnedRational) {
	h.t.Helper()
	for i, v := range h.owned {
		if v == value {
			serviceMust(h.t, h.s.Del(value))
			copy(h.owned[i:], h.owned[i+1:])
			h.owned = h.owned[:len(h.owned)-1]
			return
		}
	}
	h.t.Fatal("source named value absent from owning roots")
}
