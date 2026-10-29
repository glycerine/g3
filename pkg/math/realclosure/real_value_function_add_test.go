package realclosure

import (
	"bytes"
	"testing"
)

func TestRealValueAlgebraicPlusRationalRetainsNumeratorAndInterval(t *testing.T) {
	f := newRealFunctionFixture(t)
	out, err := f.store.addValues(f.alpha, f.one)
	mustRealFunction(t, err)
	f.own(out)
	if out.root != f.root || len(out.functionNumerator.coefficients) != 2 || f.coefficientText(out.functionNumerator.coefficients[0]) != "1" || f.coefficientText(out.functionNumerator.coefficients[1]) != "1" {
		t.Fatal("alpha+1 numerator", out.root, len(out.functionNumerator.coefficients))
	}
	var text bytes.Buffer
	mustRealFunction(t, out.functionInterval.Display(&text))
	if text.String() != "[2, 3]" {
		t.Fatalf("alpha+1 interval = %q", text.String())
	}
}
