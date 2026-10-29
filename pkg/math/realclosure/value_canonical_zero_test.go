package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestCanonicalResultDeletesExactAlgebraicZero(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	six, minusFive, one := f.rational(6), f.rational(-5), f.rational(1)
	minusTwo := f.rational(-2)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	lower, upper := numeral.NewBinaryRational(), numeral.NewBinaryRational()
	if err := f.bounds.SetInt32Power(&lower, 11, 3); err != nil { // 11/8 < sqrt(2)
		t.Fatal(err)
	}
	if err := f.bounds.SetInt32Power(&upper, 3, 1); err != nil { // 3/2 < sqrt(3)
		t.Fatal(err)
	}
	isolation, err := NewBinaryInterval(f.bounds)
	if err != nil {
		t.Fatal(err)
	}
	if err := isolation.SetFinite(&lower, &upper, true, true); err != nil {
		t.Fatal(err)
	}
	root, err := owner.mkAlgebraic([]*realValue{six, nil, minusFive, nil, one}, isolation, isolation, nil, -1)
	if err != nil {
		t.Fatal(err)
	}
	zeroApproximation, err := NewBinaryInterval(f.bounds)
	if err != nil {
		t.Fatal(err)
	}
	if err := zeroApproximation.SetFrom(f.interval); err != nil {
		t.Fatal(err)
	}
	value, err := f.store.newAlgebraicPolynomialValue(root, []*realValue{minusTwo, nil, one}, zeroApproximation)
	if err != nil {
		t.Fatal(err)
	}
	sign, err := f.store.exactSingleRootPolynomialSign(value, true)
	if err != nil || sign != 0 {
		t.Fatal(sign, err)
	}
	canonical, err := f.store.canonicalizeRetainedResult(value, true)
	if err != nil || canonical != nil || !root.deleted {
		t.Fatal(canonical, root.deleted, err)
	}
	for _, cleanupErr := range []error{zeroApproximation.Close(), isolation.Close(), f.bounds.Del(&lower), f.bounds.Del(&upper)} {
		if cleanupErr != nil {
			t.Fatal(cleanupErr)
		}
	}
}
