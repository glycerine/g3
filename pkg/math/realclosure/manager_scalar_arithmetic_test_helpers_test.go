package realclosure

import "testing"

func requireSameScalarResult(t *testing.T, store *realValueStore, got, want *realValue, usePrem bool) {
	t.Helper()
	equal, err := store.equalAnyDeterminedValues(got, want, usePrem)
	if err != nil || !equal {
		t.Fatal("scalar result mismatch", equal, err)
	}
}

func releaseScalarResults(t *testing.T, store *realValueStore, values ...*realValue) {
	t.Helper()
	for _, value := range values {
		if err := store.release(value); err != nil {
			t.Fatal(err)
		}
	}
}
