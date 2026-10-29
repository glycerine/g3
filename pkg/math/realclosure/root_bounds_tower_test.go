package realclosure

import "testing"

func TestTowerRootBoundsAcrossLowerRanks(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newTowerRootFixture(t, usePrem)
			lower, hasLower, err := f.base.store.positiveRootLowerBoundTower(f.polynomial, f.base.bounds, usePrem)
			if err != nil || !hasLower {
				t.Fatal(lower, hasLower, err)
			}
			upper, hasUpper, err := f.base.store.positiveRootUpperBoundTower(f.polynomial, f.base.bounds, usePrem)
			if err != nil || !hasUpper || lower > upper {
				t.Fatal(lower, upper, hasUpper, err)
			}
		})
	}
}
