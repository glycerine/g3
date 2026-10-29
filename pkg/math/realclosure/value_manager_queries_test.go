package realclosure

import "testing"

func TestRetainedManagerValueQueries(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one, half := f.rational(-2), f.rational(1), f.rational(1)
	if err := f.store.numbers.SetFraction(&half.rational, 1, 2); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		value                   *realValue
		zero, pos, neg, integer bool
	}{
		{nil, true, false, false, true},
		{minusTwo, false, false, true, true},
		{one, false, true, false, true},
		{half, false, true, false, false},
	} {
		zero, e0 := f.store.isZeroValue(test.value)
		pos, e1 := f.store.isPositiveValue(test.value)
		neg, e2 := f.store.isNegativeValue(test.value)
		integer, e3 := f.store.isIntegerValue(test.value)
		if e0 != nil || e1 != nil || e2 != nil || e3 != nil || zero != test.zero || pos != test.pos || neg != test.neg || integer != test.integer {
			t.Fatal("query mismatch", zero, pos, neg, integer, e0, e1, e2, e3)
		}
	}
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateSquareFreeRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	integer, err := f.store.isIntegerValue(roots[0])
	if err != nil || integer {
		t.Fatal("source algebraic integer TODO behavior", integer, err)
	}
	depends, err := f.store.dependsOnInfinitesimalsValue(roots[0])
	if err != nil || depends {
		t.Fatal("rational-coefficient algebraic dependency", depends, err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
