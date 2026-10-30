package realclosure

import "testing"

func TestNormalizedIntervalSignCountsKeepPositiveRoot(t *testing.T) {
	f := newNormalizedSignQueryFixture(t)
	equal, positive, negative, err := f.store.signCountsIntervalNormalized(f.polynomial, f.x, f.positive)
	mustRealFunction(t, err)
	if equal != 0 || positive != 1 || negative != 0 {
		t.Fatalf("interval sign counts = (%d,%d,%d), want (0,1,0)", equal, positive, negative)
	}
}
