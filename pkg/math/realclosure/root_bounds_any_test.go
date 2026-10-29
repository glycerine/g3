package realclosure

import "testing"

func TestAnyRootBoundsAlgebraicCoefficients(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnySturmFixture(t)
			lower, hasLower, err := f.base.store.positiveRootLowerBoundAny(f.polynomial, f.base.bounds, usePrem)
			if err != nil || !hasLower {
				t.Fatal("positive lower bound", lower, hasLower, err)
			}
			upper, hasUpper, err := f.base.store.positiveRootUpperBoundAny(f.polynomial, f.base.bounds, usePrem)
			if err != nil || !hasUpper || lower > upper {
				t.Fatal("positive upper bound", lower, upper, hasUpper, err)
			}
			if _, _, err := f.base.store.negativeRootLowerBoundAny(f.polynomial, f.base.bounds, usePrem); err != nil {
				t.Fatal(err)
			}
			if _, _, err := f.base.store.negativeRootUpperBoundAny(f.polynomial, f.base.bounds, usePrem); err != nil {
				t.Fatal(err)
			}
		})
	}
}
