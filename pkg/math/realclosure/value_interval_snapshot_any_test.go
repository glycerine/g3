package realclosure

import "testing"

func TestSnapshotAnyRetainedIntervalDoesNotAlias(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.store.snapshotAnyRetainedInterval(roots[1], f.bounds, 8)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.refineAnyRetainedInterval(roots[1], f.bounds, 14); err != nil {
		t.Fatal(err)
	}
	sameLower, err := f.bounds.Equal(&snapshot.lower, &roots[1].functionInterval.lower)
	if err != nil || sameLower {
		t.Fatal("snapshot aliased refinement", sameLower, err)
	}
	if err := snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
