package realclosure

import "testing"

func TestNormalizedIntervalTarskiQueryUsesAlgebraicCoefficientSigns(t *testing.T) {
	f := newNormalizedSignQueryFixture(t)
	query, err := f.store.tarskiIntervalNormalized(f.polynomial, []*realValue{f.alpha}, f.positive)
	mustRealFunction(t, err)
	if query != 1 {
		t.Fatalf("positive interval Tarski query = %d, want 1", query)
	}
}
