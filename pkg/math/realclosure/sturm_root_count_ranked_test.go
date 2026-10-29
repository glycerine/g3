package realclosure

import "testing"

func TestTowerSturmRankedRootCount(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newTowerSturmFixture(t, usePrem)
			count, err := f.base.store.rootCountBinaryIntervalRanked(
				f.sibling.root.polynomial.coefficients, f.sibling.root.isolating, usePrem)
			if err != nil || count != 1 {
				t.Fatal("ranked root count", count, err)
			}
		})
	}
}
