package realclosure

import "testing"

func TestDetermineRetainedValueSignUpdatesAmbiguousInterval(t *testing.T) {
	f, roots, difference := ambiguousSqrtDifference(t, true)
	sign, err := f.store.determineRetainedValueSign(difference, true)
	if err != nil || sign != -1 {
		t.Fatal(sign, err)
	}
	contains, err := difference.functionInterval.ContainsZero()
	if err != nil || contains {
		t.Fatal("certified interval still contains zero", contains, err)
	}
	if err := f.store.release(difference); err != nil {
		t.Fatal(err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
