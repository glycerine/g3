package realclosure

import "testing"

func TestRankedFieldPolynomialGCD(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newRankedValueFixture(t, usePrem)
			negativeGamma, err := f.base.store.negateRankedRetainedValue(f.gamma, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			factor := []*realValue{negativeGamma, f.one}
			left, err := f.base.store.multiplyRankedPolynomials(factor, []*realValue{f.alpha, f.one}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			right, err := f.base.store.multiplyRankedPolynomials(factor, []*realValue{f.one, f.one}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			gcd, err := f.base.store.gcdRankedPolynomials(left, right, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(gcd, factor)
			f.release(left, right, gcd, []*realValue{negativeGamma})
		})
	}
}
