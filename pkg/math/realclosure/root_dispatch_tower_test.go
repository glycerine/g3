package realclosure

import "testing"

func TestTowerRootDispatchEndToEnd(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newTowerRootFixture(t, usePrem)
			roots, err := f.alpha.root.owner.isolateTowerRoots(f.polynomial, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expectSignedTowerRoots(t, roots, usePrem)
			releaseIsolatedValues(t, f.base.store, roots)
		})
	}
}
