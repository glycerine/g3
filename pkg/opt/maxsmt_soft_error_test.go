package opt

import (
	"testing"
)

func TestWeightedSoftSetRejectsInvalidOwnersAndWeights(t *testing.T) {
	f := newWeightedSoftFixture(t)
	if _, err := newWeightedSoftSet(nil); err == nil {
		t.Fatal("nil manager accepted")
	}
	set, _ := newWeightedSoftSet(f.manager)
	if err := set.Add(f.p, nil); err == nil {
		t.Fatal("nil weight accepted")
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	if err := set.Add(f.p, testRatPtr(1, 1)); err == nil {
		t.Fatal("closed set accepted term")
	}
	if _, _, _, err := set.Release(); err == nil {
		t.Fatal("closed set released twice")
	}
}
