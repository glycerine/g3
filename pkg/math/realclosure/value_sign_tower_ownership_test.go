package realclosure

import "testing"

func TestTowerSturmExactZeroReleasesDifferenceDependencies(t *testing.T) {
	f := newTowerSturmFixture(t, true)
	gammaRefs, rootRefs := f.gamma.refs, f.sibling.root.refs
	difference, err := f.base.store.subtractTowerValuesUncanonicalized(f.sibling, f.gamma, true)
	if err != nil {
		t.Fatal(err)
	}
	sign, err := f.base.store.determineTowerRetainedValueSign(difference, true)
	if err != nil || sign != 0 {
		t.Fatal("tower exact zero", sign, err)
	}
	if err := f.base.store.release(difference); err != nil {
		t.Fatal(err)
	}
	if f.gamma.refs != gammaRefs || f.sibling.root.refs != rootRefs {
		t.Fatal("difference retained dependencies", gammaRefs, f.gamma.refs, rootRefs, f.sibling.root.refs)
	}
}
