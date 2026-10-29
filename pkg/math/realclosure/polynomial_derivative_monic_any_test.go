package realclosure

import "testing"

func TestAnyPolynomialDerivativeAndMonicNormalization(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnyPolynomialFixture(t)
			twoAlpha, err := f.base.store.multiplyRetainedValues(f.two, f.alpha)
			if err != nil {
				t.Fatal(err)
			}
			derivative, err := f.base.store.derivativeAnyPolynomial(
				[]*realValue{f.alpha, f.alpha, f.alpha}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(derivative, []*realValue{f.alpha, twoAlpha})

			monic, err := f.base.store.makeAnyPolynomialMonic(
				[]*realValue{f.alpha, twoAlpha}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			half, err := f.base.store.newRational(1, 2)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(monic, []*realValue{half, f.one})
			f.release([]*realValue{twoAlpha}, derivative, monic, []*realValue{half})
		})
	}
}
