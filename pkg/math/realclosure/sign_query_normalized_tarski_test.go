package realclosure

import "testing"

func TestNormalizedUnboundedTarskiQueryWithAlgebraicRoot(t *testing.T) {
	f := newNormalizedSignQueryFixture(t)
	query, err := f.store.tarskiUnboundedNormalized(f.polynomial, f.x)
	mustRealFunction(t, err)
	if query != 0 {
		t.Fatalf("unbounded Tarski query = %d, want 0", query)
	}
}
