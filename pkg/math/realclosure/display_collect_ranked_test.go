package realclosure

import "testing"

func TestRankedAPIDisplayDependencyCollection(t *testing.T) {
	f := newRankedValueFixture(t, true)
	sum, err := f.base.store.addRankedRetainedValues(f.gamma, f.alpha, true)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := f.base.store.collectRankedAlgebraicRoots(sum)
	if err != nil || len(roots) != 2 {
		t.Fatal("collected roots", len(roots), err)
	}
	if roots[0] != f.alpha.root || roots[1] != f.gamma.root || roots[0].index >= roots[1].index {
		t.Fatal("dependency order", roots[0].index, roots[1].index)
	}
	f.releaseValues(sum)
}
