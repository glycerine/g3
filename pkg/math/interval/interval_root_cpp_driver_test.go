package interval

// Source: src/test/interval.cpp tst_root_2 and tst_root_3.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/clibrand"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func sourceRootBoundRat(t *testing.T, numbers *numeral.RationalManager, value *numeral.Rational) *big.Rat {
	t.Helper()
	text, err := numbers.String(value)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		t.Fatal("invalid rational", text)
	}
	return r
}

func TestIntervalSourceSeededRootEndpoints(t *testing.T) {
	f := newRationalTranscendentalFixture(t)
	defer f.close()
	input, precision, lower, upper := f.rational("0"), f.rational("1/100"), f.rational("0"), f.rational("0")
	inputInterval, resultInterval := f.interval(), f.interval()
	root := RationalRoot{Numbers: f.number, Limit: &control.Reslimit{Limit: 1000000}}
	var rng clibrand.GLibc31
	for _, degree := range []uint32{2, 3} {
		for i := 0; i < 1000; {
			v := sourceIntervalRandom(&rng, 10)
			if degree == 2 && (v.lowerInf || v.lower < 0) {
				continue
			}
			i++
			if err := f.number.SetInt64(&inputInterval.lower, v.lower); err != nil {
				t.Fatal(err)
			}
			if err := f.number.SetInt64(&inputInterval.upper, v.upper); err != nil {
				t.Fatal(err)
			}
			inputInterval.lowerInf, inputInterval.upperInf = v.lowerInf, v.upperInf
			inputInterval.lowerOpen, inputInterval.upperOpen = v.lowerOpen, v.upperOpen
			if err := inputInterval.CheckInvariant(); err != nil {
				t.Fatalf("degree %d case %d source interval: %v", degree, i, err)
			}
			var lowerRoot, upperRoot *big.Rat
			if !v.lowerInf {
				if err := f.number.SetInt64(input, v.lower); err != nil {
					t.Fatal(err)
				}
				if err := root.Bounds(input, degree, precision, lower, upper); err != nil {
					t.Fatalf("degree %d case %d lower endpoint %d: %v", degree, i, v.lower, err)
				}
				lowerRoot = sourceRootBoundRat(t, f.number, lower)
				if err := f.number.Copy(&resultInterval.lower, lower); err != nil {
					t.Fatal(err)
				}
				lowerRootUpper := sourceRootBoundRat(t, f.number, upper)
				lowerExact := lowerRoot.Cmp(lowerRootUpper) == 0
				resultInterval.lowerInf = false
				resultInterval.lowerOpen = v.lowerOpen && lowerExact
			} else {
				resultInterval.lowerInf, resultInterval.lowerOpen = true, true
			}
			if !v.upperInf {
				if err := f.number.SetInt64(input, v.upper); err != nil {
					t.Fatal(err)
				}
				if err := root.Bounds(input, degree, precision, lower, upper); err != nil {
					t.Fatalf("degree %d case %d upper endpoint %d: %v", degree, i, v.upper, err)
				}
				upperRoot = sourceRootBoundRat(t, f.number, upper)
				if err := f.number.Copy(&resultInterval.upper, upper); err != nil {
					t.Fatal(err)
				}
				upperRootLower := sourceRootBoundRat(t, f.number, lower)
				upperExact := upperRootLower.Cmp(upperRoot) == 0
				resultInterval.upperInf = false
				resultInterval.upperOpen = v.upperOpen && upperExact
			} else {
				resultInterval.upperInf, resultInterval.upperOpen = true, true
			}
			if err := resultInterval.CheckInvariant(); err != nil {
				t.Fatalf("degree %d case %d result interval: %v", degree, i, err)
			}
			for _, sample := range v.points() {
				if lowerRoot != nil {
					lowerPower := new(big.Rat).SetInt64(1)
					for n := uint32(0); n < degree; n++ {
						lowerPower.Mul(lowerPower, lowerRoot)
					}
					if lowerPower.Cmp(sample) > 0 {
						t.Fatalf("degree %d case %d lower root bound excludes source point %s", degree, i, sample)
					}
				}
				if upperRoot != nil {
					upperPower := new(big.Rat).SetInt64(1)
					for n := uint32(0); n < degree; n++ {
						upperPower.Mul(upperPower, upperRoot)
					}
					if upperPower.Cmp(sample) < 0 {
						t.Fatalf("degree %d case %d upper root bound excludes source point %s", degree, i, sample)
					}
				}
			}
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
