package realclosure

import "testing"

func TestOuterRationalRootDispatchRemovesRepeatedFactors(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newBoundedSignFixture(t, -3, 3)
			two, minusThree, one := f.rational(2), f.rational(-3), f.rational(1)
			owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
			roots, err := owner.isolateRationalRoots([]*realValue{two, minusThree, nil, one}, usePrem)
			if err != nil || len(roots) != 2 {
				t.Fatal("repeated-root dispatch", len(roots), err)
			}
			for i, want := range []int32{-1, 1} {
				sign, signErr := f.store.signValue(roots[i])
				if signErr != nil || sign != want {
					t.Fatalf("root %d sign=%d err=%v", i, sign, signErr)
				}
			}
			releaseIsolatedValues(t, f.store, roots)
		})
	}
}

func TestOuterRationalRootDispatchCollapsesRepeatedZero(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 2)
	minusOne, one := f.rational(-1), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{nil, nil, minusOne, one}, true) // x^2(x-1)
	if err != nil || len(roots) != 2 {
		t.Fatal(len(roots), err)
	}
	if roots[0] == nil || roots[1] != nil {
		t.Fatal("nonzero root then one distinguished zero expected")
	}
	sign, err := f.store.signValue(roots[0])
	if err != nil || sign != 1 {
		t.Fatal(sign, err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
