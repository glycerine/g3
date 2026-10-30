package realclosure

import "testing"

func TestPseudoNormalizedTarskiQueryUsesAlgebraicCoefficient(t *testing.T) {
	f := newNormalizedSignQueryFixture(t)
	query, err := f.store.tarskiUnboundedPseudoNormalized(f.polynomial, f.x)
	mustRealFunction(t, err)
	if query != 0 {
		t.Fatalf("pseudo Tarski query = %d, want 0", query)
	}
}
