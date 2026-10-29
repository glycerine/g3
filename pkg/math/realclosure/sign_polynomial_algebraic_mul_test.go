package realclosure

import "testing"

func TestRealPolynomialConvolutionCarriesAlgebraicCoefficient(t *testing.T) {
	f := newRealFunctionFixture(t)
	two := f.rational(2)
	result, err := f.store.multiplyPolynomials([]*realValue{f.alpha, f.one}, []*realValue{two})
	mustRealFunction(t, err)
	t.Cleanup(func() { mustRealFunction(t, f.store.releaseTemporaryCoefficients(result)) })
	if len(result) != 2 || result[0] == nil || result[0].root != f.root || f.coefficientText(result[0].functionNumerator.coefficients[1]) != "2" || f.coefficientText(result[1]) != "2" {
		t.Fatal("algebraic convolution coefficients")
	}
}
