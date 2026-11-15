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
			if usePrem {
				// square_free under use_prem divides by the prem_gcd result,
				// which is not monic (AUDIT.md math_realclosure_01-F10); the
				// quotient is a constant multiple of (x - alpha)(x + 1).
				monic, err := f.base.store.makeAnyPolynomialMonic(squareFree, usePrem)
				if err != nil {
					t.Fatal(err)
				}
				f.expect(monic, want)
				f.release(monic)
			} else {
				f.expect(squareFree, want)
			}
			f.release(repeated, polynomial, want, squareFree, []*realValue{negativeAlpha})
		})
	}
}
