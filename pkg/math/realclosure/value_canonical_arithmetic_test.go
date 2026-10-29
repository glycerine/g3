package realclosure

import "testing"

func TestCanonicalArithmeticDeterminesAmbiguousSigns(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	threeHalves := fixtureFraction(t, f, 3, 2)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	difference, err := f.store.subtractCanonicalValues(roots[1], threeHalves, true)
	if err != nil {
		t.Fatal(err)
	}
	sign, err := f.store.signValue(difference)
	if err != nil || sign != -1 {
		t.Fatal(sign, err)
	}
	product, err := f.store.multiplyCanonicalValues(difference, roots[1], true)
	if err != nil {
		t.Fatal(err)
	}
	sign, err = f.store.signValue(product)
	if err != nil || sign != -1 {
		t.Fatal(sign, err)
	}
	sum, err := f.store.addCanonicalValues(difference, threeHalves, true)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := f.store.structurallyEqualValues(sum, roots[1])
	if err != nil || !equal {
		t.Fatal(equal, err)
	}
	for _, value := range []*realValue{difference, product, sum} {
		if err := f.store.release(value); err != nil {
			t.Fatal(err)
		}
	}
	releaseIsolatedValues(t, f.store, roots)
}
