package realclosure

import "testing"

func TestAnyPolynomialSquareFreeOverAlgebraicCoefficients(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnyPolynomialFixture(t)
			negativeAlpha, err := f.base.store.negateRetainedValue(f.alpha)
			if err != nil {
				t.Fatal(err)
			}
			factor := []*realValue{negativeAlpha, f.one}
			repeated, err := f.base.store.multiplyAnyPolynomials(factor, factor, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			polynomial, err := f.base.store.multiplyAnyPolynomials(repeated, []*realValue{f.one, f.one}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			want, err := f.base.store.multiplyAnyPolynomials(factor, []*realValue{f.one, f.one}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			squareFree, err := f.base.store.squareFreeAnyPolynomial(polynomial, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(squareFree, want)
			f.release(repeated, polynomial, want, squareFree, []*realValue{negativeAlpha})
		})
	}
}
