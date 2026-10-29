package opt

import (
	"math/big"
	"testing"
)

func TestWeightedSoftSetCloseOwnsUnreleasedReferences(t *testing.T) {
	f := newWeightedSoftFixture(t)
	set, _ := newWeightedSoftSet(f.manager)
	if err := set.Add(f.p, big.NewRat(1, 1)); err != nil {
		t.Fatal(err)
	}
	if err := set.Add(f.q, big.NewRat(2, 1)); err != nil {
		t.Fatal(err)
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	if set.Len() != 0 {
		t.Fatal("closed set retained terms")
	}
}

func TestWeightedSoftSetReleaseTransfersReferences(t *testing.T) {
	f := newWeightedSoftFixture(t)
	set, _ := newWeightedSoftSet(f.manager)
	if err := set.Add(f.p, big.NewRat(1, 1)); err != nil {
		t.Fatal(err)
	}
	terms, _, _, err := set.Release()
	if err != nil {
		t.Fatal(err)
	}
	if err = set.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = f.manager.RefGet(terms[0]); err != nil {
		t.Fatal("released reference was destroyed", err)
	}
	if err = f.manager.DelRef(&terms[0]); err != nil {
		t.Fatal(err)
	}
}
