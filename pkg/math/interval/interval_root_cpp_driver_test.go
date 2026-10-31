package interval

// Source: src/test/interval.cpp tst_root_2 and tst_root_3.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/clibrand"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestIntervalSourceSeededRootEndpoints(t *testing.T) {
	f := newRationalTranscendentalFixture(t)
	defer f.close()
	input, precision, lower, upper := f.rational("0"), f.rational("1/100"), f.rational("0"), f.rational("0")
	root := RationalRoot{Numbers: f.number, Limit: &control.Reslimit{Limit: 1000000}}
	var rng clibrand.GLibc31
	for _, degree := range []uint32{2, 3} {
		for i := 0; i < 1000; {
			v := sourceIntervalRandom(&rng, 10)
			if degree == 2 && (v.lowerInf || v.lower < 0) {
				continue
			}
			i++
			for _, endpoint := range []struct {
				value int64
				inf   bool
			}{{v.lower, v.lowerInf}, {v.upper, v.upperInf}} {
				if endpoint.inf {
					continue
				}
				if err := f.number.SetInt64(input, endpoint.value); err != nil {
					t.Fatal(err)
				}
				if err := root.Bounds(input, degree, precision, lower, upper); err != nil {
					t.Fatalf("degree %d case %d endpoint %d: %v", degree, i, endpoint.value, err)
				}
				bounds := [2]*big.Rat{}
				for j, bound := range []*numeral.Rational{lower, upper} {
					text, err := f.number.String(bound)
					if err != nil {
						t.Fatal(err)
					}
					var ok bool
					bounds[j], ok = new(big.Rat).SetString(text)
					if !ok {
						t.Fatal("invalid rational", text)
					}
				}
				lo, hi := bounds[0], bounds[1]
				loPower, hiPower := big.NewRat(1, 1), big.NewRat(1, 1)
				for n := uint32(0); n < degree; n++ {
					loPower.Mul(loPower, lo)
					hiPower.Mul(hiPower, hi)
				}
				target := new(big.Rat).SetInt64(endpoint.value)
				if lo.Cmp(hi) > 0 || degree == 2 && lo.Sign() < 0 || loPower.Cmp(target) > 0 || hiPower.Cmp(target) < 0 {
					t.Fatalf("degree %d case %d endpoint %d has invalid bounds [%s, %s]", degree, i, endpoint.value, lo, hi)
				}
			}
		}
	}
}
