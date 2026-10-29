package realclosure

import "testing"

func TestSquareFreeRationalRootDispatchOrdersAndPublishes(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newBoundedSignFixture(t, -1, 1)
			minusOne, one := f.rational(-1), f.rational(1)
			owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
			roots, err := owner.isolateSquareFreeRationalRoots([]*realValue{nil, minusOne, nil, one}, usePrem) // x(x-1)(x+1)
			if err != nil || len(roots) != 3 {
				t.Fatal("outer isolation", len(roots), err)
			}
			if roots[2] != nil {
				t.Fatal("zero root was not appended last")
			}
			for i, want := range []int32{-1, 1} {
				if roots[i] == nil {
					t.Fatalf("root %d was zero", i)
				}
				sign, err := f.store.signValue(roots[i])
				if err != nil || sign != want {
					t.Fatalf("root %d sign=%d err=%v", i, sign, err)
				}
			}
			releaseIsolatedValues(t, f.store, roots)
		})
	}
}

func TestSquareFreeRationalRootDispatchPublishesIrrationalPair(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateSquareFreeRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil || len(roots) != 2 {
		t.Fatal("sqrt pair", len(roots), err)
	}
	if roots[0] == nil || roots[1] == nil || roots[0].isRational || roots[1].isRational {
		t.Fatal("irrational roots were not algebraic values")
	}
	for i, want := range []int32{-1, 1} {
		sign, signErr := f.store.signValue(roots[i])
		if signErr != nil || sign != want {
			t.Fatalf("root %d sign=%d err=%v", i, sign, signErr)
		}
		if roots[i].root.signs != nil || roots[i].root.signIndex != -1 || roots[i].root.refs != 1 {
			t.Fatalf("root %d single-root metadata", i)
		}
	}
	releaseIsolatedValues(t, f.store, roots)
}

func TestSquareFreeRationalRootDispatchLinearAndConstant(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	two, minusSix := f.rational(2), f.rational(-6)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateSquareFreeRationalRoots([]*realValue{minusSix, two}, false)
	if err != nil || len(roots) != 1 || roots[0] == nil || !roots[0].isRational {
		t.Fatal("linear dispatch", len(roots), err)
	}
	three := f.rational(3)
	equal, err := f.store.numbers.Equal(&roots[0].rational, &three.rational)
	if err != nil || !equal {
		t.Fatal("linear root", equal, err)
	}
	releaseIsolatedValues(t, f.store, roots)
	roots, err = owner.isolateSquareFreeRationalRoots([]*realValue{two}, false)
	if err != nil || len(roots) != 0 {
		t.Fatal("constant dispatch", roots, err)
	}
}
