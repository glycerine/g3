package realclosure

import "testing"

func TestAlgebraicIntegerContentNormalizationScalesWholeFunction(t *testing.T) {
	f := newRealFunctionFixture(t)
	two := f.rational(2)
	twiceAlpha, err := f.store.multiplyNormalizedValues(two, f.alpha)
	mustRealFunction(t, err)
	four, err := f.store.newRational(4, 1)
	mustRealFunction(t, err)
	values := []*realValue{twiceAlpha, four}
	t.Cleanup(func() { mustRealFunction(t, f.store.releaseTemporaryCoefficients(values)) })
	mustRealFunction(t, f.store.normalizeIntegerContentNormalized(values))
	if values[0] == nil || values[0].isRational || len(values[0].functionNumerator.coefficients) != 2 || f.coefficientText(values[0].functionNumerator.coefficients[1]) != "1" || f.coefficientText(values[1]) != "2" {
		t.Fatal("integer content did not scale the whole algebraic function")
	}
}
