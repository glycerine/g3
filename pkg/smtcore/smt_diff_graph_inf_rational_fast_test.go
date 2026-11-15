package smtcore

import (
	"math/big"
	"math/rand"
	"testing"
)

// TestSMTInfRationalIntegralFastPaths checks the integral Add/Sub/Cmp
// shortcuts against big.Rat, including aliasing and a non-integral
// destination.
func TestSMTInfRationalIntegralFastPaths(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	pick := func() *big.Rat {
		n := new(big.Int).Rand(rng, new(big.Int).Lsh(big.NewInt(1), uint(1+rng.Intn(90))))
		if rng.Intn(2) == 0 {
			n.Neg(n)
		}
		d := big.NewInt(1)
		if rng.Intn(4) == 0 {
			d.SetInt64(int64(2 + rng.Intn(9)))
		}
		return new(big.Rat).SetFrac(n, d)
	}
	for i := 0; i < 4000; i++ {
		a, b, z := pick(), pick(), pick()
		for _, sub := range []bool{false, true} {
			want := new(big.Rat)
			if sub {
				want.Sub(a, b)
			} else {
				want.Add(a, b)
			}
			got := new(big.Rat).Set(z)
			smtRatAddSub(got, a, b, sub)
			if got.Cmp(want) != 0 || got.String() != want.String() {
				t.Fatal(a, b, z, sub, got, want)
			}
			alias := new(big.Rat).Set(a)
			smtRatAddSub(alias, alias, b, sub)
			if alias.Cmp(want) != 0 || alias.String() != want.String() {
				t.Fatal("alias", a, b, sub, alias, want)
			}
		}
		if smtRatCmp(a, b) != a.Cmp(b) || smtRatCmp(a, a) != 0 {
			t.Fatal("cmp", a, b)
		}
	}
}
