package realclosure

import "testing"

func TestRealValueAlgebraicInverseUsesExtendedPolynomialGCD(t *testing.T) {
	f := newRealFunctionFixture(t)
	inverse, err := f.store.inverseNormalizedValue(f.alpha)
	mustRealFunction(t, err)
	f.own(inverse)
	if inverse == nil || inverse.isRational || len(inverse.functionNumerator.coefficients) != 2 || inverse.functionNumerator.coefficients[0] != nil || f.coefficientText(inverse.functionNumerator.coefficients[1]) != "1/2" {
		t.Fatal("inverse alpha numerator was not x/2")
	}
	identity, err := f.store.multiplyNormalizedValues(f.alpha, inverse)
	mustRealFunction(t, err)
	f.own(identity)
	if identity == nil || !identity.isRational || f.coefficientText(identity) != "1" {
		t.Fatal("alpha times inverse alpha was not one")
	}
}
