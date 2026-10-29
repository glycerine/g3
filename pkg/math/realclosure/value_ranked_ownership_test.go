package realclosure

import "testing"

func TestRankedValueResultOwnsLowerCoefficientAndHigherRoot(t *testing.T) {
	f := newRankedValueFixture(t, true)
	alphaRefs, rootRefs := f.alpha.refs, f.gamma.root.refs
	sum, err := f.base.store.addRankedRetainedValues(f.gamma, f.alpha, true)
	if err != nil {
		t.Fatal(err)
	}
	if f.alpha.refs != alphaRefs+1 || f.gamma.root.refs != rootRefs+1 {
		t.Fatal("ranked result ownership", alphaRefs, f.alpha.refs, rootRefs, f.gamma.root.refs)
	}
	f.releaseValues(sum)
	if f.alpha.refs != alphaRefs || f.gamma.root.refs != rootRefs {
		t.Fatal("ranked result retained dependencies after release", alphaRefs, f.alpha.refs, rootRefs, f.gamma.root.refs)
	}
}
