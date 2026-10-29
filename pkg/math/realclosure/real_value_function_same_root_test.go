package realclosure

import "testing"

func TestRealValueSameExtensionAdditionCombinesNumerators(t *testing.T) {
	f := newRealFunctionFixture(t)
	out, err := f.store.addValues(f.alpha, f.alpha)
	mustRealFunction(t, err)
	f.own(out)
	if len(out.functionNumerator.coefficients) != 2 || out.functionNumerator.coefficients[0] != nil || f.coefficientText(out.functionNumerator.coefficients[1]) != "2" {
		t.Fatal("alpha+alpha numerator")
	}
	if f.root.refs != 2 {
		t.Fatal("same-root result did not retain exactly one root reference", f.root.refs)
	}
}
