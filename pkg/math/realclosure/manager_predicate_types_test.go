package realclosure

import "testing"

func TestManagerNumeralPredicatesRetainedTypes(t *testing.T) {
	f := newRankedValueFixture(t, true)
	for _, tc := range []struct {
		value               *realValue
		rational, algebraic bool
	}{
		{nil, true, false},
		{f.one, true, false},
		{f.alpha, false, true},
		{f.gamma, false, true},
	} {
		rational, e0 := f.base.store.queryValueIsRational(tc.value)
		algebraic, e1 := f.base.store.queryValueIsAlgebraic(tc.value)
		infinitesimal, e2 := f.base.store.queryValueIsInfinitesimal(tc.value)
		transcendental, e3 := f.base.store.queryValueIsTranscendental(tc.value)
		if e0 != nil || e1 != nil || e2 != nil || e3 != nil || rational != tc.rational || algebraic != tc.algebraic || infinitesimal || transcendental {
			t.Fatal(rational, algebraic, infinitesimal, transcendental, e0, e1, e2, e3)
		}
	}
}
