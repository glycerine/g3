package realclosure

import "testing"

func TestRealValueNormalizedInverseKeepsRationalFastPath(t *testing.T) {
	f := newRealFunctionFixture(t)
	three := f.rational(3)
	inverse, err := f.store.inverseNormalizedValue(three)
	mustRealFunction(t, err)
	f.own(inverse)
	if inverse == nil || !inverse.isRational || f.coefficientText(inverse) != "1/3" {
		t.Fatal("rational inverse fast path")
	}
}
