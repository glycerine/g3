package interval

// Source: src/test/trigo.cpp tst_e.

import (
	"math/big"
	"testing"
)

func TestTranscendentalSourceEPrecisions(t *testing.T) {
	f := newRationalTranscendentalFixture(t)
	defer f.close()
	// The first 150 factorial terms give a rational lower bound; the
	// remaining tail is strictly less than 1/(149*149!).
	sum := new(big.Rat)
	factorial := big.NewInt(1)
	for n := int64(0); n < 150; n++ {
		if n > 0 {
			factorial.Mul(factorial, big.NewInt(n))
		}
		sum.Add(sum, new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Set(factorial)))
	}
	upper := new(big.Rat).Add(sum, new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Mul(big.NewInt(149), factorial)))
	loRef, hiRef := f.rational(sum.RatString()), f.rational(upper.RatString())
	result := f.interval()
	for precision := uint32(0); precision < 64; precision++ {
		if err := f.owner.E(precision, result); err != nil {
			t.Fatal(precision, err)
		}
		for _, reference := range []struct {
			value *big.Rat
			lower bool
		}{{sum, true}, {upper, false}} {
			value := hiRef
			if reference.lower {
				value = loRef
			}
			if ok, err := result.Contains(value); err != nil || !ok {
				t.Fatal("e bound", precision, reference.value, err)
			}
		}
	}
}
