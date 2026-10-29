package realclosure

import "testing"

func TestExactBinaryFallbackPreservesCoefficientOwners(t *testing.T) {
	f := newExactBinaryFallbackFixture(t, true)
	alphaRefs, oneRefs := f.alpha.refs, f.one.refs
	value, err := f.base.store.exactPolynomialValueAtBinaryBound([]*realValue{f.one, f.alpha}, f.alpha.root.owner.intervals, &f.oneBound, true)
	if err != nil {
		t.Fatal(err)
	}
	f.releaseValues(value)
	if f.alpha.refs != alphaRefs || f.one.refs != oneRefs {
		t.Fatal("result release changed inputs", alphaRefs, f.alpha.refs, oneRefs, f.one.refs)
	}
}
