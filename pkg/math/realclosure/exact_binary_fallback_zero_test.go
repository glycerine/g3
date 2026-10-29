package realclosure

import "testing"

func TestExactBinaryFallbackCancelsAtIntegerBound(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		f := newExactBinaryFallbackFixture(t, usePrem)
		negativeAlpha, err := f.base.store.negateRankedRetainedValue(f.alpha, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		sign, err := f.base.store.exactSignAtBinaryBound([]*realValue{negativeAlpha, f.alpha}, f.alpha.root.owner.intervals, &f.oneBound, usePrem)
		if err != nil || sign != 0 {
			t.Fatal(usePrem, sign, err)
		}
		f.releaseValues(negativeAlpha)
	}
}
