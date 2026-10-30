package realclosure

import "testing"

func TestRealValueAlgebraicConjugateProductCanonicalizesRational(t *testing.T) {
	f := newRealFunctionFixture(t)
	plus, err := f.store.addValues(f.alpha, f.one)
	mustRealFunction(t, err)
	f.own(plus)
	minus, err := f.store.subtractValues(f.alpha, f.one)
	mustRealFunction(t, err)
	f.own(minus)
	out, err := f.store.multiplyNormalizedValues(plus, minus)
	mustRealFunction(t, err)
	f.own(out)
	if out == nil || !out.isRational || f.coefficientText(out) != "1" {
		t.Fatal("(alpha+1)(alpha-1) did not reduce to 1")
	}
}
