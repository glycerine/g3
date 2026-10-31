package interval

// Source: src/test/trigo.cpp tst_e.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/clibrand"
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

func TestTranscendentalSourceSineCosineSamples(t *testing.T) {
	f := newRationalTranscendentalFixture(t)
	defer f.close()
	input, lower, upper := f.rational("0"), f.rational("0"), f.rational("0")
	refLower, refUpper := f.rational("0"), f.rational("0")
	// The source runs two 100-sample floating sine passes before the
	// rational cases; each sample consumes two C rand values.
	var rng clibrand.GLibc31
	for i := 0; i < 400; i++ {
		rng.Next()
	}
	for _, cosine := range []bool{false, true} {
		for _, precision := range []uint32{3, 5, 9} {
			for sample := 0; sample < 40; sample++ {
				n := int64(4 * (rng.Next() % 100000))
				if rng.Next()%2 == 0 {
					n = -n
				}
				x := new(big.Rat).SetFrac(big.NewInt(n), big.NewInt(100000))
				if err := f.number.SetString(input, x.RatString()); err != nil {
					t.Fatal(err)
				}
				var err error
				if cosine {
					err = f.owner.Cosine(input, precision, lower, upper)
				} else {
					err = f.owner.Sine(input, precision, lower, upper)
				}
				if err != nil {
					t.Fatal(err)
				}
				// Consecutive high-order alternating Taylor partial sums enclose
				// the function because the tail terms decrease for |x| <= 4.
				x2 := new(big.Rat).Mul(x, x)
				term := new(big.Rat).SetInt64(1)
				if !cosine {
					term.Set(x)
				}
				sum := new(big.Rat).Set(term)
				for j := int64(0); j < 40; j++ {
					a, b := 2*j+1, 2*j+2
					if !cosine {
						a, b = 2*j+2, 2*j+3
					}
					term.Mul(term, x2)
					term.Quo(term, new(big.Rat).SetInt64(a*b))
					term.Neg(term)
					sum.Add(sum, term)
				}
				a, b := int64(81), int64(82)
				if !cosine {
					a, b = 82, 83
				}
				next := new(big.Rat).Mul(term, x2)
				next.Quo(next, new(big.Rat).SetInt64(a*b))
				next.Neg(next)
				other := new(big.Rat).Add(sum, next)
				if sum.Cmp(other) > 0 {
					sum, other = other, sum
				}
				if err = f.number.SetString(refLower, sum.RatString()); err != nil {
					t.Fatal(err)
				}
				if err = f.number.SetString(refUpper, other.RatString()); err != nil {
					t.Fatal(err)
				}
				lo, err := f.number.Compare(lower, refLower)
				if err != nil {
					t.Fatal(err)
				}
				hi, err := f.number.Compare(refUpper, upper)
				if err != nil {
					t.Fatal(err)
				}
				if lo > 0 || hi > 0 {
					t.Fatalf("cosine=%t k=%d x=%s bounds exclude independent Taylor enclosure", cosine, precision, x)
				}
			}
			// Advance past the remaining 160 samples in this source pass.
			for i := 0; i < 320; i++ {
				rng.Next()
			}
		}
	}
}
