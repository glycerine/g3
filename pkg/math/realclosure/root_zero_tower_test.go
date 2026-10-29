package realclosure

import "testing"

func TestTowerRootDispatchDistinguishedZeroAndOwnership(t *testing.T) {
	f := newTowerRootFixture(t, true)
	shifted := append([]*realValue{nil}, f.polynomial...)
	alphaRefs, gammaRefs := f.alpha.refs, f.gamma.refs
	roots, err := f.alpha.root.owner.isolateTowerRoots(shifted, true)
	if err != nil || len(roots) != 3 || roots[2] != nil {
		t.Fatal(roots, err)
	}
	f.expectSignedTowerRoots(t, roots[:2], true)
	releaseIsolatedValues(t, f.base.store, roots)
	if f.alpha.refs != alphaRefs || f.gamma.refs != gammaRefs {
		t.Fatal("tower isolation retained lower roots", alphaRefs, f.alpha.refs, gammaRefs, f.gamma.refs)
	}
}
