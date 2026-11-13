package numeral

import (
	"math"
	"math/big"
	"math/bits"
)

// SetSmallRat sets v to a reduced Go rational whose numerator and
// denominator both fit in int32, without the decimal round trip of
// SetString. It reports false, changing nothing, when the value is out of
// that range or when the mpq_set trace that SetString emits is enabled; the
// caller then uses SetString. For such values both paths store the same
// normalized small numerator and denominator.
func (s *RationalService) SetSmallRat(v *OwnedRational, r *big.Rat) (bool, error) {
	if r == nil {
		return false, nil
	}
	num, den := r.Num(), r.Denom()
	if !num.IsInt64() || !den.IsInt64() {
		return false, nil
	}
	n, d := num.Int64(), den.Int64()
	if n <= math.MinInt32 || n > math.MaxInt32 || d <= 0 || d > math.MaxInt32 {
		return false, nil
	}
	if s == nil || s.Manager == nil || s.Manager.parseTraceTags().ShouldTrace("mpq_set") {
		return false, nil
	}
	if d == 1 {
		return true, s.SetInt64(v, n)
	}
	return true, s.SetFractionSigned(v, int32(n), int32(d))
}

// SetBigRat sets v to a reduced Go rational of any size from its binary
// limbs. SetString's decimal parse does a multiply-add per digit and was
// quadratic in the digit count on large model values (fuzz seed q_9009). A
// big.Rat is reduced with a positive denominator, so the result is the same
// normalized value SetString stores. Like SetSmallRat it reports false,
// changing nothing, when the mpq_set trace that SetString emits is enabled.
func (s *RationalService) SetBigRat(v *OwnedRational, r *big.Rat) (bool, error) {
	if r == nil || s == nil || s.Manager == nil || s.Manager.parseTraceTags().ShouldTrace("mpq_set") {
		return false, nil
	}
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.check(v); e != nil {
		return false, e
	}
	z := s.Manager.Z
	if e := z.SetLimbs(&v.Raw.Num, bigIntLimbs32(r.Num())); e != nil {
		return false, e
	}
	if r.Sign() < 0 {
		if e := z.Neg(&v.Raw.Num); e != nil {
			return false, e
		}
	}
	if e := z.SetLimbs(&v.Raw.Den, bigIntLimbs32(r.Denom())); e != nil {
		return false, e
	}
	return true, nil
}

// bigIntLimbs32 returns |x| as little-endian 32-bit digits.
func bigIntLimbs32(x *big.Int) []uint32 {
	words := x.Bits()
	out := make([]uint32, 0, 2*len(words))
	for _, w := range words {
		out = append(out, uint32(w))
		if bits.UintSize == 64 {
			out = append(out, uint32(uint64(w)>>32))
		}
	}
	return out
}
