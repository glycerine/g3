package realclosure

import "testing"

func TestRealValueAlgebraicDivisionPublishesReducedQuotient(t *testing.T) {
	f := newRealFunctionFixture(t)
	plus, err := f.store.addValues(f.alpha, f.one)
	mustRealFunction(t, err)
	f.own(plus)
	quotient, err := f.store.divideNormalizedValues(plus, f.alpha)
	mustRealFunction(t, err)
	f.own(quotient)
	if quotient == nil || quotient.isRational || len(quotient.functionNumerator.coefficients) != 2 || f.coefficientText(quotient.functionNumerator.coefficients[0]) != "1" || f.coefficientText(quotient.functionNumerator.coefficients[1]) != "1/2" {
		t.Fatal("(alpha+1)/alpha numerator")
	}
}
