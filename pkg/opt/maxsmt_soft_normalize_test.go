package opt

import (
	"testing"
)

func TestWeightedSoftSetNormalizesNegativeWeightAndOffset(t *testing.T) {
	f := newWeightedSoftFixture(t)
	set, err := newWeightedSoftSet(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	if err = set.Add(f.p, testRatPtr(-7, 2)); err != nil {
		t.Fatal(err)
	}
	terms, weights, offset, err := set.Release()
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 1 || ratString(&weights[0]) != "7/2" || ratString(&offset) != "-7/2" {
		t.Fatalf("weights %v offset %s", weights, ratString(&offset))
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
	if err := set.Add(f.p, testRatPtr(0, 1)); err != nil {
		t.Fatal(err)
	}
	if set.Len() != 0 {
		t.Fatal("zero weight created a term")
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
}
