package realclosure

import "testing"

func TestRankedValueNegationAndExactCancellation(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newRankedValueFixture(t, usePrem)
			negative, err := f.base.store.negateRankedRetainedValue(f.gamma, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			minusOne := f.base.rational(-1)
			f.expect(negative.functionNumerator.coefficients, []*realValue{nil, minusOne})
			zero, err := f.base.store.subtractRankedRetainedValues(f.gamma, f.gamma, usePrem)
			if err != nil || zero != nil {
				t.Fatal("ranked cancellation", zero, err)
			}
			f.releaseValues(negative)
		})
	}
}
