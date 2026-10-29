package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestExactDivideAlgebraicValueByInteger(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one, six := f.rational(-2), f.rational(1), f.rational(6)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	sixAlpha, err := f.store.multiplyRetainedValues(roots[1], six)
	if err != nil {
		t.Fatal(err)
	}
	three := numeral.NewInt(3)
	quotient, err := f.store.exactDivideValueByInteger(sixAlpha, &three, true)
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, quotient.functionNumerator.coefficients, 0, 2)
	if err := f.store.release(quotient); err != nil {
		t.Fatal(err)
	}
	if err := f.store.release(sixAlpha); err != nil {
		t.Fatal(err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
