package sat

import "testing"

func TestSATExternalWatchPreservesConstraintIndex(t *testing.T) {
	for _, want := range []uint32{0, 3, 42, ^uint32(0)} {
		watch, err := newSATExternalWatch(want)
		satOK(t, err)
		got, err := watch.ExternalIndex()
		satOK(t, err)
		if got != want {
			t.Fatal(got, want)
		}
	}
}
