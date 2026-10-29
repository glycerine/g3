package realclosure

import "testing"

func TestAnyRootBisectionPublishesNestedRoots(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnySturmFixture(t)
			owner := &algebraicRootOwner{values: f.base.store, intervals: f.base.bounds}
			lower, hasLower, err := f.base.store.positiveRootLowerBoundAny(f.polynomial, f.base.bounds, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			upper, hasUpper, err := f.base.store.positiveRootUpperBoundAny(f.polynomial, f.base.bounds, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			bounded, err := makeRootBoundInterval(f.base.bounds, lower, hasLower, upper, hasUpper, false)
			if err != nil {
				t.Fatal(err)
			}
			defer bounded.Close()
			half, err := makeRootBoundInterval(f.base.bounds, 0, false, 0, false, false)
			if err != nil {
				t.Fatal(err)
			}
			defer half.Close()
			roots, err := owner.bisectAnyRoots(f.polynomial, bounded, half, 2, usePrem)
			if err != nil || len(roots) != 2 {
				t.Fatal("bisected roots", len(roots), err)
			}
			nested := 0
			for _, root := range roots {
				if root.isRational || root.root == nil || len(root.root.polynomial.coefficients) != len(f.polynomial) {
					t.Fatal("missing nested defining polynomial")
				}
				nested++
			}
			if nested != 2 {
				t.Fatal("missing nested roots", nested)
			}
			releaseIsolatedValues(t, f.base.store, roots)
		})
	}
}
