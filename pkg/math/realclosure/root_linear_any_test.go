package realclosure

import "testing"

func TestAnyRootLinearAlgebraicCoefficient(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnySturmFixture(t)
			owner := &algebraicRootOwner{values: f.base.store, intervals: f.base.bounds}
			roots, err := owner.isolateNonzeroSquareFreeAnyRoots(f.query, usePrem)
			if err != nil || len(roots) != 1 {
				t.Fatal("linear roots", len(roots), err)
			}
			equal, err := f.base.store.structurallyEqualValues(roots[0], f.alpha)
			if err != nil || !equal {
				t.Fatal("linear root did not remain in existing extension", equal, err)
			}
			releaseIsolatedValues(t, f.base.store, roots)
		})
	}
}
