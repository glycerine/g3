package realclosure

import "testing"

func TestNormalizedIntervalRootCountIncludesAndExcludesAlgebraicRoot(t *testing.T) {
	f := newNormalizedSignQueryFixture(t)
	count, err := f.store.rootCountIntervalNormalized(f.polynomial, f.positive)
	mustRealFunction(t, err)
	if count != 1 {
		t.Fatalf("positive interval root count = %d, want 1", count)
	}
	count, err = f.store.rootCountIntervalNormalized(f.polynomial, f.enclosure)
	mustRealFunction(t, err)
	if count != 0 {
		t.Fatalf("refined alpha enclosure root count = %d, want 0", count)
	}
}
