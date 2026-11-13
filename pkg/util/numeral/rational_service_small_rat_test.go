package numeral

import (
	"math"
	"math/big"
	"testing"
)

// SetSmallRat must store exactly what SetString stores for the same value.
func TestRationalServiceSetSmallRatMatchesSetString(t *testing.T) {
	s, _ := serviceNew(t)
	values := []int64{0, 1, -1, 2, -2, 3, 7, 10, -10, 12, 1000, -65536, 65537, math.MaxInt32, math.MinInt32 + 1, math.MaxInt32 - 1}
	for _, n := range values {
		for _, d := range []int64{1, 2, 3, 7, 10, 65536, math.MaxInt32} {
			r := new(big.Rat).SetFrac64(n, d)
			fast, e := s.NewValue()
			serviceMust(t, e)
			slow, e := s.NewValue()
			serviceMust(t, e)
			done, e := s.SetSmallRat(&fast, r)
			serviceMust(t, e)
			serviceMust(t, s.SetString(&slow, r.RatString()))
			if !done {
				t.Fatal("small rational took the slow path", r)
			}
			if fast.Raw.Num.Cell != 0 || slow.Raw.Num.Cell != 0 || fast.Raw.Den.Cell != 0 || slow.Raw.Den.Cell != 0 || fast.Raw != slow.Raw {
				t.Fatal("representation differs", r, fast.Raw, slow.Raw)
			}
			serviceMust(t, s.Del(&fast))
			serviceMust(t, s.Del(&slow))
		}
	}
	big1 := new(big.Rat).SetFrac64(math.MaxInt32+1, 1)
	v, e := s.NewValue()
	serviceMust(t, e)
	if done, e := s.SetSmallRat(&v, big1); done || e != nil {
		t.Fatal("out-of-range numerator must take the slow path", done, e)
	}
	serviceMust(t, s.Del(&v))
}
