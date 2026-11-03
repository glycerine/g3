package rel

import (
	"errors"
	"testing"
)

func TestFiniteCheckedPairLazyUnionDeltaLifecycleAndMismatch(t *testing.T) {
	f := newTableRelationFixture(t)
	lazy := NewLazySparseFiniteFactory()
	target := newCheckedLazyInterpretedPair(t, f, lazy, Fact{0})
	source := newCheckedLazyInterpretedPair(t, f, lazy, Fact{1})
	delta := newCheckedLazyInterpretedPair(t, f, lazy)

	astTestMust(t, target.UnionFrom(source, delta))
	astTestMust(t, source.Close())
	if rows, err := target.Facts(); err != nil || len(rows) != 2 {
		t.Fatal("checked lazy union target after source close", rows, err)
	}
	if found, err := delta.Contains(Fact{1}); err != nil || !found {
		t.Fatal("checked lazy union delta after source close", found, err)
	}
	primary, checker, err := target.ConcreteKinds()
	if err != nil || primary != "lazy_sparse" || checker != "sparse" {
		t.Fatal("checked lazy union changed concrete kinds", primary, checker, err)
	}
	astTestMust(t, errors.Join(target.Close(), delta.Close()))

	corrupt := newCheckedLazyInterpretedPair(t, f, lazy, Fact{0})
	cleanSource := newCheckedLazyInterpretedPair(t, f, lazy, Fact{1})
	cleanDelta := newCheckedLazyInterpretedPair(t, f, lazy)
	if inserted, e := corrupt.primary.(*LazySparseTable).Add(Fact{1}); e != nil || !inserted {
		t.Fatal("checked lazy union corruption setup", inserted, e)
	}
	if err = corrupt.UnionFrom(cleanSource, cleanDelta); !errors.Is(err, ErrCheckTable) {
		t.Fatal("checked lazy union accepted mismatched target", err)
	}
	if rows, e := cleanDelta.Facts(); e != nil || len(rows) != 0 {
		t.Fatal("checked lazy union mutated delta before mismatch rejection", rows, e)
	}
	if removed, e := corrupt.primary.(*LazySparseTable).Remove(Fact{1}); e != nil || !removed {
		t.Fatal("checked lazy union corruption repair", removed, e)
	}
	astTestMust(t, errors.Join(corrupt.Close(), cleanSource.Close(), cleanDelta.Close(), lazy.Close()))
}
