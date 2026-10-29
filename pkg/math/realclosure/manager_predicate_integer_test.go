package realclosure

import "testing"

func TestManagerNumeralPredicatesIntegerAndDependency(t *testing.T) {
	f := newRankedValueFixture(t, true)
	half, err := f.base.store.newRational(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		value   *realValue
		integer bool
	}{
		{nil, true}, {f.two, true}, {half, false}, {f.alpha, false},
	} {
		integer, e0 := f.base.store.queryValueIsInteger(tc.value)
		depends, e1 := f.base.store.queryValueDependsOnInfinitesimals(tc.value)
		if e0 != nil || e1 != nil || integer != tc.integer || depends {
			t.Fatal(integer, depends, e0, e1)
		}
	}
	f.releaseValues(half)
}
