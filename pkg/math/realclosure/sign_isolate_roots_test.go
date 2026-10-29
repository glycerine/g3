package realclosure

import "testing"

func releaseIsolatedValues(t *testing.T, store *realValueStore, values []*realValue) {
	t.Helper()
	for _, value := range values {
		if err := store.release(value); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRationalSignIsolationPublishesThreeSharedRoots(t *testing.T) {
	for _, usePrem := range []bool{true, false} {
		t.Run(map[bool]string{true: "pseudo", false: "remainder"}[usePrem], func(t *testing.T) {
			f := newBoundedSignFixture(t, 0, 4)
			one, minusSix, eleven := f.rational(1), f.rational(-6), f.rational(11)
			p := []*realValue{minusSix, eleven, minusSix, one} // (x-1)(x-2)(x-3)
			owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
			values, err := owner.isolateRationalRootsBySigns(p, 3, f.interval, f.interval, usePrem)
			if err != nil || len(values) != 3 || len(owner.roots) != 3 {
				t.Fatal("three-root publication", len(values), len(owner.roots), err)
			}
			det := owner.roots[0].signs
			if det == nil || det.refs != 3 || len(det.qs) != 1 || len(det.prs) != 3 || det.matrix.rows != 3 {
				t.Fatal("three roots do not share completed sign state")
			}
			for i, value := range values {
				root := owner.roots[i]
				if value.root != root || root.signs != det || root.signIndex != i || root.refs != 1 || len(value.functionNumerator.coefficients) != 2 || value.functionNumerator.coefficients[0] != nil {
					t.Fatalf("published root %d state", i)
				}
				if count, err := root.numRootsInsideInterval(); err != nil || count != 3 {
					t.Fatalf("root %d count=%d, err=%v", i, count, err)
				}
			}
			releaseIsolatedValues(t, f.store, values[:2])
			if det.deleted || det.refs != 1 {
				t.Fatal("shared sign state died before last root")
			}
			releaseIsolatedValues(t, f.store, values[2:])
			if !det.deleted || det.refs != 0 {
				t.Fatal("last root retained sign state")
			}
		})
	}
}

func TestRationalSignIsolationFourRootDerivativeLoop(t *testing.T) {
	for _, usePrem := range []bool{true, false} {
		t.Run(map[bool]string{true: "pseudo", false: "remainder"}[usePrem], func(t *testing.T) {
			f := newBoundedSignFixture(t, 0, 5)
			one, minusTen, thirtyFive, minusFifty, twentyFour := f.rational(1), f.rational(-10), f.rational(35), f.rational(-50), f.rational(24)
			p := []*realValue{twentyFour, minusFifty, thirtyFive, minusTen, one} // (x-1)(x-2)(x-3)(x-4)
			owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
			values, err := owner.isolateRationalRootsBySigns(p, 4, f.interval, f.interval, usePrem)
			if err != nil || len(values) != 4 || len(owner.roots) != 4 {
				t.Fatal("four-root publication", len(values), len(owner.roots), err)
			}
			det := owner.roots[0].signs
			if det == nil || det.refs != 4 || len(det.qs) != 2 || len(det.prs) != 4 || det.matrix.rows != 4 || len(det.conditions) != 4 {
				t.Fatal("derivative loop did not reach four singleton conditions")
			}
			// Reverse scanning skips the constant derivative and retains two
			// actual discriminators before all cardinalities reach one.
			if len(det.qs[0].coefficients) != 2 || len(det.qs[1].coefficients) != 3 {
				t.Fatal("unexpected retained derivative order", len(det.qs[0].coefficients), len(det.qs[1].coefficients))
			}
			releaseIsolatedValues(t, f.store, values)
			if !det.deleted {
				t.Fatal("four-root sign owner survived all values")
			}
		})
	}
}

func TestAlgebraicRootUnpublishedRegistrationCleanup(t *testing.T) {
	f := newBoundedSignFixture(t, 0, 4)
	one, minusSix, eleven := f.rational(1), f.rational(-6), f.rational(11)
	p := []*realValue{minusSix, eleven, minusSix, one}
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	root, err := owner.mkAlgebraic(p, f.interval, f.interval, nil, -1)
	if err != nil {
		t.Fatal(err)
	}
	if root.refs != 0 || len(owner.roots) != 1 || owner.roots[0] != root {
		t.Fatal("unpublished root registration prefix")
	}
	if err := root.discardUnpublished(); err != nil {
		t.Fatal(err)
	}
	if !root.deleted || owner.roots[0] != nil || root.polynomial.coefficients != nil || root.interval != nil || root.isolating != nil {
		t.Fatal("unpublished root cleanup did not reverse registration")
	}
}
