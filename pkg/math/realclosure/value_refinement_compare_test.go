package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestAlgebraicValueRefinementCloneAndComparison(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateSquareFreeRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil || len(roots) != 2 {
		t.Fatal(len(roots), err)
	}
	clone, err := f.store.cloneValueInterval(roots[1], f.bounds, 12)
	if err != nil {
		t.Fatal(err)
	}
	widthPrecise := func() bool {
		width := numeral.NewBinaryRational()
		if err := f.bounds.Sub(&clone.upper, &clone.lower, &width); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.bounds.Del(&width) }()
		precise, err := f.bounds.LessOneDiv2k(&width, 12)
		if err != nil {
			t.Fatal(err)
		}
		return precise
	}()
	if !widthPrecise {
		t.Fatal("clone was not refined")
	}
	if err := clone.Close(); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		a, b *realValue
		want int
	}{
		{roots[0], nil, -1}, {nil, roots[1], -1}, {roots[0], roots[1], -1},
		{roots[1], roots[0], 1}, {roots[1], roots[1], 0}, {one, roots[1], -1},
	} {
		got, err := f.store.compareValues(test.a, test.b, f.bounds)
		if err != nil || got != test.want {
			t.Fatalf("compare got=%d want=%d err=%v", got, test.want, err)
		}
	}
	less, err := f.store.lessValues(roots[0], roots[1], f.bounds)
	if err != nil || !less {
		t.Fatal("less wrapper", less, err)
	}
	greater, err := f.store.greaterValues(roots[1], one, f.bounds)
	if err != nil || !greater {
		t.Fatal("greater wrapper", greater, err)
	}
	equal, err := f.store.equalValues(roots[1], roots[1], f.bounds)
	if err != nil || !equal {
		t.Fatal("equal wrapper", equal, err)
	}
	releaseIsolatedValues(t, f.store, roots)
}

func TestAlgebraicRootMetadataQueries(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateSquareFreeRationalRoots([]*realValue{minusTwo, nil, one}, false)
	if err != nil {
		t.Fatal(err)
	}
	root := roots[0].root
	index, err := root.registryIndex()
	if err != nil || index != 0 {
		t.Fatal(index, err)
	}
	degree, err := root.definingDegree()
	if err != nil || degree != 2 {
		t.Fatal(degree, err)
	}
	signIndex, shared, err := root.signConditionIndex()
	if err != nil || shared || signIndex != -1 {
		t.Fatal(signIndex, shared, err)
	}
	interval, err := root.cloneDefiningInterval()
	if err != nil {
		t.Fatal(err)
	}
	if err := interval.Close(); err != nil {
		t.Fatal(err)
	}
	releaseIsolatedValues(t, f.store, roots)
	if _, err := root.registryIndex(); err == nil {
		t.Fatal("deleted root metadata remained queryable")
	}
}
