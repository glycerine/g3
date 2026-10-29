package realclosure

import "testing"

func TestTowerRootBisectionAcrossLowerRanks(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newTowerRootFixture(t, usePrem)
			lower, hasLower, err := f.base.store.positiveRootLowerBoundTower(f.polynomial, f.base.bounds, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			upper, hasUpper, err := f.base.store.positiveRootUpperBoundTower(f.polynomial, f.base.bounds, usePrem)
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
			roots, err := f.alpha.root.owner.bisectTowerRoots(f.polynomial, bounded, half, 1, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			if len(roots) != 1 {
				t.Fatal("positive roots", len(roots))
			}
			positive, err := f.base.store.isPositiveRankedValue(roots[0], usePrem)
			if err != nil || !positive {
				t.Fatal("positive tower root", positive, err)
			}
			releaseIsolatedValues(t, f.base.store, roots)
		})
	}
}
