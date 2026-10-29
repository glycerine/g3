package realclosure

import "testing"

func TestRefineAndCertifyGeneralAlgebraicValue(t *testing.T) {
	f, roots, difference := ambiguousSqrtDifference(t, true)
	sign, refined, err := f.store.refineAndCertifyRetainedValue(difference, f.bounds, 12, true)
	if err != nil || !refined || sign != -1 {
		t.Fatal(sign, refined, err)
	}
	precise, err := f.store.retainedIntervalPrecise(difference, f.bounds, 12)
	if err != nil || !precise {
		t.Fatal(precise, err)
	}
	if err := f.store.release(difference); err != nil {
		t.Fatal(err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
