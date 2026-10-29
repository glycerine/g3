package realclosure

import "testing"

func TestRankedFieldPolynomialLongDivision(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newRankedValueFixture(t, usePrem)
			negativeGamma, err := f.base.store.negateRankedRetainedValue(f.gamma, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			negativeAlpha, err := f.base.store.negateRankedRetainedValue(f.alpha, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			factor := []*realValue{negativeGamma, f.one}
			other := []*realValue{negativeAlpha, f.one}
			product, err := f.base.store.multiplyRankedPolynomials(factor, other, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			quotient, remainder, err := f.base.store.divideRankedPolynomials(product, factor, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(quotient, other)
			if len(remainder) != 0 {
				t.Fatal("nonzero exact remainder", remainder)
			}
			f.release(product, quotient, remainder, []*realValue{negativeGamma, negativeAlpha})
		})
	}
}
