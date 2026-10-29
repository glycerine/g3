package realclosure

import "testing"

func TestRetainedValueStructuralEquality(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	firstHalf := fixtureFraction(t, f, 1, 2)
	secondHalf := fixtureFraction(t, f, 2, 4)
	equal, err := f.store.structurallyEqualValues(firstHalf, secondHalf)
	if err != nil || !equal {
		t.Fatal(equal, err)
	}
	equal, err = f.store.structurallyEqualValues(nil, nil)
	if err != nil || !equal {
		t.Fatal(equal, err)
	}
	equal, err = f.store.structurallyEqualValues(nil, firstHalf)
	if err != nil || equal {
		t.Fatal(equal, err)
	}
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	alias, err := f.store.newAlgebraicValue(roots[0].root, one)
	if err != nil {
		t.Fatal(err)
	}
	equal, err = f.store.structurallyEqualValues(roots[0], alias)
	if err != nil || !equal {
		t.Fatal("same extension numerator-x", equal, err)
	}
	equal, err = f.store.structurallyEqualValues(roots[0], roots[1])
	if err != nil || equal {
		t.Fatal("different extensions", equal, err)
	}
	if err := f.store.release(alias); err != nil {
		t.Fatal(err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
