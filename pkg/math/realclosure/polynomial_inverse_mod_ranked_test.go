package realclosure

import "testing"

func TestRankedFieldPolynomialModularInverse(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newRankedValueFixture(t, usePrem)
			modulus := f.gamma.root.polynomial.coefficients
			inverse, coprime, err := f.base.store.invertRankedPolynomialModulo([]*realValue{nil, f.one}, modulus, usePrem)
			if err != nil || !coprime {
				t.Fatal("modular inverse", coprime, err)
			}
			product, err := f.base.store.multiplyRankedPolynomials([]*realValue{nil, f.one}, inverse, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			quotient, remainder, err := f.base.store.divideRankedPolynomials(product, modulus, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(remainder, []*realValue{f.one})
			f.release(inverse, product, quotient, remainder)
		})
	}
}
