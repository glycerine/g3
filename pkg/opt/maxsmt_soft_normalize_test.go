package opt

import (
	"math/big"
	"testing"
)

func TestWeightedSoftSetNormalizesNegativeWeightAndOffset(t *testing.T) {
	f := newWeightedSoftFixture(t)
	set, err := newWeightedSoftSet(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	if err = set.Add(f.p, big.NewRat(-7, 2)); err != nil {
		t.Fatal(err)
	}
	terms, weights, offset, err := set.Release()
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 1 || weights[0].RatString() != "7/2" || offset.RatString() != "-7/2" {
		t.Fatalf("weights %v offset %s", weights, offset.RatString())
	}
	h, err := f.manager.RefGet(terms[0])
	if err != nil {
		t.Fatal(err)
	}
	yes, argument, err := f.manager.IsNot(h)
	if err != nil || !yes || argument != f.p {
		t.Fatalf("normalized NOT matched=%v argument=%v error %v", yes, argument, err)
	}
	if err = f.manager.DelRef(&terms[0]); err != nil {
		t.Fatal(err)
	}
}

func TestWeightedSoftSetIgnoresZeroWeight(t *testing.T) {
	f := newWeightedSoftFixture(t)
	set, _ := newWeightedSoftSet(f.manager)
	if err := set.Add(f.p, new(big.Rat)); err != nil {
		t.Fatal(err)
	}
	if set.Len() != 0 {
		t.Fatal("zero weight created a term")
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
}
