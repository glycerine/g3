package realclosure

import "testing"

func TestAnyPolynomialScalarDivideAndNegate(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnyPolynomialFixture(t)
			twoAlpha, err := f.base.store.multiplyRetainedValues(f.two, f.alpha)
			if err != nil {
				t.Fatal(err)
			}
			four := f.base.rational(4)
			divided, err := f.base.store.divideAnyPolynomialByScalar(
				[]*realValue{twoAlpha, four}, f.two, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(divided, []*realValue{f.alpha, f.two})

			negated, err := f.base.store.negateAnyPolynomial(divided)
			if err != nil {
				t.Fatal(err)
			}
			negativeAlpha, err := f.base.store.negateRetainedValue(f.alpha)
			if err != nil {
				t.Fatal(err)
			}
			minusTwo := f.base.rational(-2)
			f.expect(negated, []*realValue{negativeAlpha, minusTwo})
			f.release([]*realValue{twoAlpha}, divided, negated, []*realValue{negativeAlpha})
		})
	}
}
