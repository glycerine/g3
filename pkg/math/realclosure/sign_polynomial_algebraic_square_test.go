package realclosure

import "testing"

func TestRealPolynomialConvolutionReducesAlgebraicSquare(t *testing.T) {
	f := newRealFunctionFixture(t)
	result, err := f.store.multiplyNormalizedPolynomials([]*realValue{f.alpha}, []*realValue{f.alpha})
	mustRealFunction(t, err)
	t.Cleanup(func() { mustRealFunction(t, f.store.releaseTemporaryCoefficients(result)) })
	if len(result) != 1 || result[0] == nil || !result[0].isRational || f.coefficientText(result[0]) != "2" {
		t.Fatal("algebraic polynomial convolution did not reduce alpha squared")
	}
}
