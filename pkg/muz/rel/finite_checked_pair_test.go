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

func TestFiniteCheckedPairCompiledOperations(t *testing.T) {
	signature, err := NewSignature([]uint64{3, 3})
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
	left := makePair("sparse", "hashtable", Fact{0, 1}, Fact{2, 2})
	right := makePair("sparse", "hashtable", Fact{1, 0}, Fact{2, 1})
	defer func() { must(t, left.Close()); must(t, right.Close()) }()
	if rows, err := left.SizeEstimateRows(); err != nil || rows != 2 {
		t.Fatal("checked row estimate", rows, err)
	}
	if bytes, err := left.SizeEstimateBytes(); err != nil || bytes == 0 {
		t.Fatal("checked byte estimate", bytes, err)
	}

	empty, err := left.EmptyLike()
	must(t, err)
	if rows, err := empty.Len(); err != nil || rows != 0 {
		t.Fatal("empty checked pair", rows, err)
	}
	must(t, empty.Close())

	joined, err := left.Join(right, []int{1}, []int{0})
	must(t, err)
	if rows, err := joined.Len(); err != nil || rows != 2 {
		t.Fatal("checked join", rows, err)
	}
	must(t, joined.Close())

	joined, err = left.JoinProject(right, []int{1}, []int{0}, []int{1, 2})
	must(t, err)
	if rows, err := joined.Len(); err != nil || rows != 2 {
		t.Fatal("checked join-project", rows, err)
	}
	must(t, joined.Close())

	projected, err := left.Project([]int{1})
	must(t, err)
	if rows, err := projected.Len(); err != nil || rows != 2 {
		t.Fatal("checked project", rows, err)
	}
	must(t, projected.Close())

	renamed, err := left.RenameCycle([]int{0, 1})
	must(t, err)
	if found, err := renamed.Contains(Fact{1, 0}); err != nil || !found {
		t.Fatal("checked rename", found, err)
	}
	must(t, renamed.Close())

	selected, err := left.SelectEqualAndProject(0, 0)
	must(t, err)
	if found, err := selected.Contains(Fact{1}); err != nil || !found {
		t.Fatal("checked select-project", found, err)
	}
	must(t, selected.Close())

	equal := makePair("sparse", "hashtable", Fact{0, 1}, Fact{2, 2})
	must(t, equal.FilterEqual(2, 0))
	if rows, err := equal.Len(); err != nil || rows != 1 {
		t.Fatal("checked filter equal", rows, err)
	}
	must(t, equal.FilterIdentical([]int{0, 1}))
	if rows, err := equal.Len(); err != nil || rows != 1 {
		t.Fatal("checked filter identical", rows, err)
	}
	must(t, equal.Close())

	negative := makePair("sparse", "hashtable", Fact{2, 0})
	filtered := makePair("sparse", "hashtable", Fact{0, 1}, Fact{2, 2})
	must(t, filtered.FilterByNegation(negative, []int{0}, []int{0}))
	if rows, err := filtered.Len(); err != nil || rows != 1 {
		t.Fatal("checked negation", rows, err)
	}
	must(t, negative.Close())
	must(t, filtered.Close())
}

func TestFiniteCheckedPairEmptyFullAndComplement(t *testing.T) {
	signature, err := NewSignature([]uint64{3})
	must(t, err)
	source, err := NewSparseTable(signature, 0)
	must(t, err)
	_, err = source.Add(Fact{1})
	must(t, err)
	pair, err := NewFiniteCheckedPair(source, "sparse")
	must(t, err)
	must(t, source.Close())
	defer func() { must(t, pair.Close()) }()

	complement, err := pair.Complement()
	must(t, err)
	defer func() { must(t, complement.Close()) }()
	for _, fact := range []Fact{{0}, {2}} {
		found, err := complement.Contains(fact)
		if err != nil || !found {
			t.Fatal("checked complement", fact, found, err)
		}
	}
	if found, err := complement.Contains(Fact{1}); err != nil || found {
		t.Fatal("checked complement retained source", found, err)
	}

	full, err := pair.FullLike()
	must(t, err)
	defer func() { must(t, full.Close()) }()
	if rows, err := full.Len(); err != nil || rows != 3 {
		t.Fatal("checked full", rows, err)
	}
}
