package realclosure

import "testing"

func TestExactBinaryFallbackCancelsAtDyadicBound(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		f := newExactBinaryFallbackFixture(t, usePrem)
		negativeHalfAlpha, err := f.base.store.negateRankedRetainedValue(f.halfAlpha, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		sign, err := f.base.store.exactSignAtBinaryBound([]*realValue{negativeHalfAlpha, f.alpha}, f.alpha.root.owner.intervals, &f.halfBound, usePrem)
		if err != nil || sign != 0 {
			t.Fatal(usePrem, sign, err)
		}
		f.releaseValues(negativeHalfAlpha)
	}
}
