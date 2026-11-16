package numeral

// Source: src/util/mpf.cpp:1438-1485.

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestArbitraryFloatRemIndependentIEEEAndAlias(t *testing.T) {
	m, _, _ := mpfNumeralHelpersFixture(t)
	rng := rand.New(rand.NewChaCha8([32]byte{14, 38, 14, 85}))
	for sample := 0; sample < 100; sample++ {
		a := math.Ldexp(1+float64(rng.Float64()), rng.IntN(601)-300)
		b := math.Ldexp(1+float64(rng.Float64()), rng.IntN(601)-300)
		if sample%2 != 0 {
			a = -a
		}
		if sample%3 != 0 {
			b = -b
		}
		if sample < 8 {
			a = []float64{3, 5, 7, -3, -5, -7, 0, math.Copysign(0, -1)}[sample]
			b = 2
		}
		for alias := 0; alias < 3; alias++ {
			var x, y, separate ArbitraryFloat
			serviceMust(t, m.SetDouble(&x, 11, 53, a))
			serviceMust(t, m.SetDouble(&y, 11, 53, b))
			beforeX, beforeY := x, y
			out := &separate
			if alias == 1 {
				out = &x
			}
			if alias == 2 {
				out = &y
			}
			serviceMust(t, m.Rem(&x, &y, out))
			got, e := m.ToDouble(out)
			serviceMust(t, e)
			want := math.Remainder(a, b)
			if math.Float64bits(got) != math.Float64bits(want) {
				t.Fatal("independent IEEE remainder", sample, alias, a, b, got, want)
			}
			if alias != 1 && x != beforeX || alias != 2 && y != beforeY {
				t.Fatal("const operand changed")
			}
			serviceMust(t, m.Del(&separate))
			serviceMust(t, m.Del(&x))
			serviceMust(t, m.Del(&y))
		}
	}
}
