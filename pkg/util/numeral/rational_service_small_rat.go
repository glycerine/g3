package numeral

import (
	"math"
	"math/big"
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
