package opt

import (
	"math/big"
	"testing"
)

func TestWeightedSoftSetCombinesDuplicatePositiveTerms(t *testing.T) {
	f := newWeightedSoftFixture(t)
	set, _ := newWeightedSoftSet(f.manager)
	for _, weight := range []*big.Rat{big.NewRat(2, 1), big.NewRat(5, 3)} {
		if err := set.Add(f.p, weight); err != nil {
			t.Fatal(err)
		}
	}
	terms, weights, _, err := set.Release()
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 1 || weights[0].RatString() != "11/3" {
		t.Fatalf("terms %d weights %v", len(terms), weights)
	}
	if err = f.manager.DelRef(&terms[0]); err != nil {
		t.Fatal(err)
	}
}

func TestWeightedSoftSetCombinesHashConsedNegativeTerms(t *testing.T) {
	f := newWeightedSoftFixture(t)
	set, _ := newWeightedSoftSet(f.manager)
	for _, weight := range []*big.Rat{big.NewRat(-2, 1), big.NewRat(-3, 1)} {
		if err := set.Add(f.q, weight); err != nil {
			t.Fatal(err)
		}
	}
	terms, weights, offset, err := set.Release()
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 1 || weights[0].RatString() != "5" || offset.RatString() != "-5" {
		t.Fatalf("terms %d weights %v offset %s", len(terms), weights, offset.RatString())
	}
	if err = f.manager.DelRef(&terms[0]); err != nil {
		t.Fatal(err)
	}
}
