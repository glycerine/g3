package realclosure

import "testing"

func TestSturmSequenceWithAlgebraicCoefficient(t *testing.T) {
	f := newRealFunctionFixture(t)
	rows, err := f.store.sturmSequenceNormalized(
		[]*realValue{f.one, nil, f.one},
		[]*realValue{f.alpha, f.one},
	)
	mustRealFunction(t, err)
	t.Cleanup(func() { mustRealFunction(t, f.store.closePolynomialSequence(rows)) })
	if len(rows) != 3 || len(rows[2].coefficients) != 1 || rows[2].coefficients[0] == nil || f.coefficientText(rows[2].coefficients[0]) != "-3" {
		t.Fatal("algebraic Sturm sequence rows")
	}
}
