package rel

import (
	"errors"
	"testing"
)

func TestFiniteCheckedPairPreservesConfiguredConcreteKinds(t *testing.T) {
	signature, err := NewSignature([]uint64{3})
	must(t, err)
	for _, tc := range []struct{ primary, checker string }{
		{"hashtable", "hashtable"}, {"hashtable", "sparse"},
		{"sparse", "hashtable"}, {"sparse", "sparse"},
	} {
		t.Run(tc.primary+"-"+tc.checker, func(t *testing.T) {
			source, err := newFiniteTableKind(tc.primary, signature)
			must(t, err)
			defer func() { must(t, source.Close()) }()
			_, err = source.Add(Fact{0})
			must(t, err)
			pair, err := NewFiniteCheckedPair(source, tc.checker)
			must(t, err)
			defer func() { must(t, pair.Close()) }()
			primary, checker, err := pair.ConcreteKinds()
			if err != nil || primary != tc.primary || checker != tc.checker {
				t.Fatal("configured kinds", primary, checker, err)
			}
			added, err := pair.Add(Fact{2})
			if err != nil || !added {
				t.Fatal("paired add", added, err)
			}
			clone, err := pair.CloneFinite()
			must(t, err)
			defer func() { must(t, clone.Close()) }()
			if rows, err := clone.Len(); err != nil || rows != 2 {
				t.Fatal("paired clone", rows, err)
			}
		})
	}
}

func TestFiniteCheckedPairDetectsConcreteMismatch(t *testing.T) {
	signature, err := NewSignature([]uint64{2})
	must(t, err)
	source := NewHashTable(signature)
	defer func() { must(t, source.Close()) }()
	pair, err := NewFiniteCheckedPair(source, "hashtable")
	must(t, err)
	defer func() { must(t, pair.Close()) }()
	_, err = pair.primary.Add(Fact{1})
	must(t, err)
	if _, err = pair.Facts(); !errors.Is(err, ErrCheckTable) {
		t.Fatal("concrete mismatch was not detected", err)
	}
}

func TestFiniteCheckedPairUnionDeltaAndOwnership(t *testing.T) {
	signature, err := NewSignature([]uint64{3})
	must(t, err)
	makePair := func(kind, checker string, facts ...Fact) *FiniteCheckedPair {
		source, err := newFiniteTableKind(kind, signature)
		must(t, err)
		for _, fact := range facts {
			_, err = source.Add(fact)
			must(t, err)
		}
		pair, err := NewFiniteCheckedPair(source, checker)
		must(t, err)
		must(t, source.Close())
		return pair
	}
	target := makePair("sparse", "sparse", Fact{0})
	source := makePair("sparse", "sparse", Fact{2})
	delta := makePair("sparse", "sparse", Fact{1})
	defer func() { must(t, target.Close()); must(t, source.Close()); must(t, delta.Close()) }()
	must(t, target.UnionFrom(source, delta))
	for _, fact := range []Fact{{0}, {2}} {
		found, err := target.Contains(fact)
		if err != nil || !found {
			t.Fatal("union target", fact, found, err)
		}
	}
	for _, fact := range []Fact{{1}, {2}} {
		found, err := delta.Contains(fact)
		if err != nil || !found {
			t.Fatal("union delta", fact, found, err)
		}
	}
	must(t, target.UnionFrom(source, delta))
	if rows, err := delta.Len(); err != nil || rows != 2 {
		t.Fatal("repeated union changed cumulative delta", rows, err)
	}
}
