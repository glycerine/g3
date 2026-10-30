package realclosure

import "testing"

func TestNormalizedSignCountsSeparatePositiveAndNegativeRoots(t *testing.T) {
	f := newNormalizedSignQueryFixture(t)
	equal, positive, negative, err := f.store.signCountsUnboundedNormalized(f.polynomial, f.x)
	mustRealFunction(t, err)
	if equal != 0 || positive != 1 || negative != 1 {
		t.Fatalf("sign counts = (%d,%d,%d), want (0,1,1)", equal, positive, negative)
	}
}
