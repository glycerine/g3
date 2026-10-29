package realclosure

import "testing"

func TestRealPolynomialDerivativeScalesAlgebraicCoefficient(t *testing.T) {
	f := newRealFunctionFixture(t)
	two := f.rational(2)
	result, err := f.store.derivativePolynomial([]*realValue{f.one, f.alpha, two})
	mustRealFunction(t, err)
	t.Cleanup(func() { mustRealFunction(t, f.store.releaseTemporaryCoefficients(result)) })
	if len(result) != 2 || result[0] == nil || result[0].root != f.root || f.coefficientText(result[0].functionNumerator.coefficients[1]) != "1" || f.coefficientText(result[1]) != "4" {
		t.Fatal("algebraic derivative coefficients")
	}
}
