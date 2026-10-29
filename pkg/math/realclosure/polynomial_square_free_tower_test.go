package realclosure

import "testing"

func TestTowerRootSquareFreeRepeatedPolynomial(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newTowerRootFixture(t, usePrem)
			repeated, err := f.base.store.multiplyRankedPolynomials(f.polynomial, f.polynomial, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			squareFree, err := f.base.store.squareFreeTowerPolynomial(repeated, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(squareFree, f.polynomial)
			f.release(repeated, squareFree)
		})
	}
}
