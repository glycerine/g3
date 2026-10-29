package realclosure

import "testing"

func TestRankedFieldPolynomialArithmetic(t *testing.T) {
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
			product, err := f.base.store.multiplyRankedPolynomials(
				[]*realValue{negativeGamma, f.one}, []*realValue{negativeAlpha, f.one}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			gammaAlpha, err := f.base.store.multiplyRankedRetainedValues(f.gamma, f.alpha, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			negativeSum, err := f.base.store.addRankedRetainedValues(negativeGamma, negativeAlpha, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(product, []*realValue{gammaAlpha, negativeSum, f.one})
			f.release(product, []*realValue{negativeGamma, negativeAlpha, gammaAlpha, negativeSum})
		})
	}
}
