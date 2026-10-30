package realclosure

import "testing"

func TestRealValueAlgebraicSquareReducesToRational(t *testing.T) {
	f := newRealFunctionFixture(t)
	out, err := f.store.multiplyNormalizedValues(f.alpha, f.alpha)
	mustRealFunction(t, err)
	f.own(out)
	if out == nil || !out.isRational || f.coefficientText(out) != "2" {
		t.Fatal("alpha squared did not reduce to 2")
	}
	if f.root.refs != 1 {
		t.Fatal("rational reduction retained an extra root reference", f.root.refs)
	}
}
