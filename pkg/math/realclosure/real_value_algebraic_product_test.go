package realclosure

import (
	"bytes"
	"testing"
)

func TestRealValueAlgebraicProductPublishesReducedNumeratorAndInterval(t *testing.T) {
	f := newRealFunctionFixture(t)
	plus, err := f.store.addValues(f.alpha, f.one)
	mustRealFunction(t, err)
	f.own(plus)
	out, err := f.store.multiplyNormalizedValues(plus, f.alpha)
	mustRealFunction(t, err)
	f.own(out)
	if out == nil || out.isRational || len(out.functionNumerator.coefficients) != 2 || f.coefficientText(out.functionNumerator.coefficients[0]) != "2" || f.coefficientText(out.functionNumerator.coefficients[1]) != "1" {
		t.Fatal("(alpha+1)*alpha reduced numerator")
	}
	var text bytes.Buffer
	mustRealFunction(t, out.functionInterval.Display(&text))
	if text.String() != "[2, 6]" {
		t.Fatalf("reduced product interval = %q", text.String())
	}
}
