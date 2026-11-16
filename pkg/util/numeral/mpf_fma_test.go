package numeral

// Source: src/util/mpf.cpp:748-949; src/util/mpf.h:129.

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestArbitraryFloatFMAIndependentIEEEAndAllAliases(t *testing.T) {
	m, _, _ := mpfNumeralHelpersFixture(t)
	triples := [][3]float64{{1 + math.Ldexp(1, -52), 1 - math.Ldexp(1, -52), -1}, {2, 3, -6}, {2, 3, 1}, {-2, 3, 1}, {2, -3, -1}, {math.SmallestNonzeroFloat64, 2, math.SmallestNonzeroFloat64}, {1, 1, math.SmallestNonzeroFloat64}}
	rng := rand.New(rand.NewChaCha8([32]byte{74, 8, 94, 9}))
	for sample := 0; sample < 150; sample++ {
		var triple [3]float64
		for i := range triple {
			triple[i] = math.Ldexp(1+float64(float64(rng.Uint64()>>11)*math.Ldexp(1, -53)), rng.IntN(401)-200)
			if rng.IntN(2) == 0 {
				triple[i] = -triple[i]
			}
		}
		triples = append(triples, triple)
	}
	for _, v := range triples {
		for alias := 0; alias < 4; alias++ {
			var x, y, z, separate ArbitraryFloat
			for i, p := range []*ArbitraryFloat{&x, &y, &z} {
				serviceMust(t, m.SetDouble(p, 11, 53, v[i]))
			}
			oldX, oldY, oldZ := x, y, z
			out := []*ArbitraryFloat{&separate, &x, &y, &z}[alias]
			serviceMust(t, m.FMA(ArbitraryFloatNearestEven, &x, &y, &z, out))
			got, e := m.ToDouble(out)
			serviceMust(t, e)
			want := math.FMA(v[0], v[1], v[2])
			if math.Float64bits(got) != math.Float64bits(want) {
				t.Fatal("independent fused IEEE result", v, alias, got, want)
			}
			if alias != 1 && x != oldX || alias != 2 && y != oldY || alias != 3 && z != oldZ {
				t.Fatal("const input changed")
			}
			for _, p := range []*ArbitraryFloat{&separate, &x, &y, &z} {
				serviceMust(t, m.Del(p))
			}
		}
	}
}
