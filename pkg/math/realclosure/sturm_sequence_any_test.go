package realclosure

import "testing"

func TestAnySturmSequenceAlgebraicCoefficients(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnySturmFixture(t)
			rows, err := f.base.store.rootSturmSequenceAny(f.polynomial, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 3 || len(rows[0].coefficients) != 3 || len(rows[1].coefficients) != 2 || len(rows[2].coefficients) != 1 {
				t.Fatalf("unexpected row degrees: %#v", rows)
			}
			sign, err := f.base.store.determineAnyRetainedValueSign(rows[2].coefficients[0], usePrem)
			if err != nil || sign != 1 {
				t.Fatal("constant remainder sign", sign, err)
			}
			if err := f.base.store.closePolynomialSequence(rows); err != nil {
				t.Fatal(err)
			}
		})
	}
}
