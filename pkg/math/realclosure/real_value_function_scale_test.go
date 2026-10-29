package realclosure

import (
	"bytes"
	"testing"
)

func TestRealValueAlgebraicRationalScaleBothOrders(t *testing.T) {
	f := newRealFunctionFixture(t)
	three := f.rational(3)
	for _, operands := range [][2]*realValue{{f.alpha, three}, {three, f.alpha}} {
		out, err := f.store.multiplyValues(operands[0], operands[1])
		mustRealFunction(t, err)
		f.own(out)
		if len(out.functionNumerator.coefficients) != 2 || out.functionNumerator.coefficients[0] != nil || f.coefficientText(out.functionNumerator.coefficients[1]) != "3" {
			t.Fatal("3*alpha numerator")
		}
		var text bytes.Buffer
		mustRealFunction(t, out.functionInterval.Display(&text))
		if text.String() != "[3, 6]" {
			t.Fatalf("3*alpha interval = %q", text.String())
		}
	}
}
