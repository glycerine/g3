package realclosure

import "testing"

func TestPseudoNormalizedRootCountUsesAlgebraicCoefficient(t *testing.T) {
	f := newNormalizedSignQueryFixture(t)
	count, err := f.store.rootCountUnboundedPseudoNormalized(f.polynomial)
	mustRealFunction(t, err)
	if count != 2 {
		t.Fatalf("pseudo root count = %d, want 2", count)
	}
}
