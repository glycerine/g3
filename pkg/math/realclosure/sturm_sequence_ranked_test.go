package realclosure

import "testing"

func TestTowerSturmSequenceSpansLowerExtensions(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newTowerSturmFixture(t, usePrem)
			query := f.siblingDifferencePolynomial(t, usePrem)
			rows, err := f.base.store.tarskiSturmSequenceRanked(f.sibling.root.polynomial.coefficients, query, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) < 2 || len(rows[0].coefficients) != 3 {
				t.Fatal("ranked Sturm rows", len(rows))
			}
			if err := f.base.store.closePolynomialSequence(rows); err != nil {
				t.Fatal(err)
			}
			f.release(query)
		})
	}
}
