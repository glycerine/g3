package realclosure

import "testing"

func TestAnyPolynomialMultiplyAlgebraicFactors(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnyPolynomialFixture(t)
			negativeAlpha, err := f.base.store.negateRetainedValue(f.alpha)
			if err != nil {
				t.Fatal(err)
			}
			product, err := f.base.store.multiplyAnyPolynomials(
				[]*realValue{f.alpha, f.one}, []*realValue{negativeAlpha, f.one}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			minusTwo := f.base.rational(-2)
			f.expect(product, []*realValue{minusTwo, nil, f.one})
			f.release(product, []*realValue{negativeAlpha})
		})
	}
}
