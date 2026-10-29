package realclosure

import "testing"

func TestRankedValueAdditionPromotesLowerExtension(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newRankedValueFixture(t, usePrem)
			sum, err := f.base.store.addRankedRetainedValues(f.gamma, f.alpha, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			if sum == nil || sum.root != f.gamma.root {
				t.Fatal("sum was not represented in higher extension")
			}
			f.expect(sum.functionNumerator.coefficients, []*realValue{f.alpha, f.one})
			f.releaseValues(sum)
		})
	}
}
