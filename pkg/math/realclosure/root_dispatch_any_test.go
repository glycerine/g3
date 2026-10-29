package realclosure

import "testing"

func TestAnyRootDispatchCleansAndSquareFrees(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnySturmFixture(t)
			owner := &algebraicRootOwner{values: f.base.store, intervals: f.base.bounds}
			repeated, err := f.base.store.multiplyAnyPolynomials(f.polynomial, f.polynomial, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			roots, err := owner.isolateAnyRoots(repeated, usePrem)
			if err != nil || len(roots) != 2 {
				t.Fatal("outer dispatch", len(roots), err)
			}
			releaseIsolatedValues(t, f.base.store, roots)
			f.release(repeated)
		})
	}
}
