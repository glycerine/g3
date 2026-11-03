package rel

import (
	"errors"
	"testing"
)

func TestCheckedExplanationIndexProjectFinitePreservesPairAndSource(t *testing.T) {
	signature, err := NewSignature([]uint64{4, 16})
	if err != nil {
		t.Fatal(err)
	}
	lazy := NewLazySparseFiniteFactory()
	factory, err := NewFiniteCheckedFactoryWithLazy("lazy_sparse", "sparse", lazy)
	if err != nil {
		t.Fatal(err)
	}
	index, err := factory.NewExplanationIndex(signature, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []Fact{{1, 5}, {2, 7}} {
		if err = index.EnsureFact(row); err != nil {
			t.Fatal(err)
		}
	}
	projected, err := index.ProjectFinite()
	if err != nil {
		t.Fatal(err)
	}
	primary, checker, err := projected.ConcreteKinds()
	if err != nil || primary != "lazy_sparse" || checker != "sparse" {
		t.Fatal("projected kinds", primary, checker, err)
	}
	rows, err := projected.Facts()
	if err != nil || len(rows) != 2 || len(rows[0]) != 1 || rows[0][0] != 1 || rows[1][0] != 2 {
		t.Fatal("projected rows", rows, err)
	}
	probe := Fact{1, 0}
	if found, e := index.FetchFact(probe); e != nil || !found || probe[1] != 5 {
		t.Fatal("projection mutated source", probe, found, e)
	}
	if err = errors.Join(projected.Close(), index.Close(), lazy.Close()); err != nil {
		t.Fatal(err)
	}
}

func TestCheckedExplanationIndexProjectFiniteNullaryAndMismatch(t *testing.T) {
	signature, err := NewSignature([]uint64{16})
	if err != nil {
		t.Fatal(err)
	}
	index, err := directCheckedExplanationIndex("sparse", "hashtable", nil, signature)
	if err != nil {
		t.Fatal(err)
	}
	if err = index.EnsureFact(Fact{3}); err != nil {
		t.Fatal(err)
	}
	projected, err := index.ProjectFinite()
	if err != nil {
		t.Fatal(err)
	}
	primary, checker, err := projected.ConcreteKinds()
	if err != nil || primary != "hashtable" || checker != "hashtable" {
		t.Fatal("nullary projected kinds", primary, checker, err)
	}
	rows, err := projected.Facts()
	if err != nil || len(rows) != 1 || len(rows[0]) != 0 {
		t.Fatal("nullary projected rows", rows, err)
	}
	if err = projected.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = index.checker.Add(Fact{4}); err != nil {
		t.Fatal(err)
	}
	if projected, err = index.ProjectFinite(); !errors.Is(err, ErrCheckTable) || projected != nil {
		t.Fatal("mismatched input projection", projected, err)
	}
	// Repair the deliberate corruption so normal ownership cleanup remains
	// independently observable.
	if _, err = index.checker.(*HashTable).Remove(Fact{4}); err != nil {
		t.Fatal(err)
	}
	if err = index.Close(); err != nil {
		t.Fatal(err)
	}
}
