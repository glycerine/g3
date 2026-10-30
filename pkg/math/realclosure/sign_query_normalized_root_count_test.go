package realclosure

import "testing"

func TestNormalizedUnboundedRootCountWithAlgebraicCoefficient(t *testing.T) {
	f := newNormalizedSignQueryFixture(t)
	count, err := f.store.rootCountUnboundedNormalized(f.polynomial)
	mustRealFunction(t, err)
	if count != 2 {
		t.Fatalf("unbounded root count = %d, want 2", count)
	}
}
