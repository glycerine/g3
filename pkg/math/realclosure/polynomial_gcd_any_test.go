package realclosure

import "testing"

func TestAnyPolynomialGCDOverAlgebraicCoefficients(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnyPolynomialFixture(t)
			negativeAlpha, err := f.base.store.negateRetainedValue(f.alpha)
			if err != nil {
				t.Fatal(err)
			}
			factor := []*realValue{negativeAlpha, f.one}
			two := f.base.rational(2)
			left, err := f.base.store.multiplyAnyPolynomials(factor, []*realValue{f.one, f.one}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			right, err := f.base.store.multiplyAnyPolynomials(factor, []*realValue{two, f.one}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			gcd, err := f.base.store.gcdAnyPolynomials(left, right, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(gcd, factor)
			f.release(left, right, gcd, []*realValue{negativeAlpha})
		})
	}
}
