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
	// f.alpha isolates sqrt(2) in [1,2], while x²-alpha has the root
	// 2^(1/4), which is also inside that interval.
	if count != 1 {
		t.Fatalf("alpha enclosure root count = %d, want 1", count)
	}
}
