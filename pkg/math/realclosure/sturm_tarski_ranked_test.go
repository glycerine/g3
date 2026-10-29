package realclosure

import "testing"

func TestTowerSturmTarskiSiblingZero(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newTowerSturmFixture(t, usePrem)
			query := f.siblingDifferencePolynomial(t, usePrem)
			got, err := f.base.store.tarskiBinaryIntervalRanked(
				f.sibling.root.polynomial.coefficients, query, f.sibling.root.isolating, usePrem)
			if err != nil || got != 0 {
				t.Fatal("sibling zero Tarski query", got, err)
			}
			f.release(query)
		})
	}
}
