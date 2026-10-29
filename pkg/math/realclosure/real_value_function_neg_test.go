package realclosure

import (
	"bytes"
	"testing"
)

func TestRealValueAlgebraicNegationAndSubtraction(t *testing.T) {
	f := newRealFunctionFixture(t)
	negative, err := f.store.negateValue(f.alpha)
	mustRealFunction(t, err)
	f.own(negative)
	if len(negative.functionNumerator.coefficients) != 2 || negative.functionNumerator.coefficients[0] != nil || f.coefficientText(negative.functionNumerator.coefficients[1]) != "-1" {
		t.Fatal("negated alpha numerator")
	}
	var text bytes.Buffer
	mustRealFunction(t, negative.functionInterval.Display(&text))
	if text.String() != "[-2, -1]" {
		t.Fatalf("negated alpha interval = %q", text.String())
	}
	difference, err := f.store.subtractValues(f.alpha, f.alpha)
	mustRealFunction(t, err)
	if difference != nil {
		f.own(difference)
		t.Fatal("alpha-alpha was not canonical zero")
	}
}
