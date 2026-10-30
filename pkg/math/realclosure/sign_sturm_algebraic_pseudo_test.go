package realclosure

import "testing"

func TestPseudoSturmSequenceWithAlgebraicCoefficientsTerminates(t *testing.T) {
	f := newNormalizedSignQueryFixture(t)
	derivative, err := f.store.derivativePolynomial(f.polynomial)
	mustRealFunction(t, err)
	rows, err := f.store.sturmSequencePseudoNormalized(f.polynomial, derivative)
	releaseErr := f.store.releaseTemporaryCoefficients(derivative)
	mustRealFunction(t, err)
	mustRealFunction(t, releaseErr)
	t.Cleanup(func() { mustRealFunction(t, f.store.closePolynomialSequence(rows)) })
	if len(rows) != 3 || len(rows[2].coefficients) != 1 {
		t.Fatal("pseudo-Sturm sequence did not terminate at a constant")
	}
}
