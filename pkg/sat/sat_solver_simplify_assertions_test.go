package sat

import "testing"

func TestSATSimplifyRunsSourceChecksAfterProbingAndAsymm(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	debug, _, _ := satIntegrityActionFixture(t, "")
	satOK(t, debug.EnableDebug("sat_missed_prop"))
	satOK(t, debug.EnableDebug("sat_simplify_bug"))
	before := s.propagation.limit.Steps
	satOK(t, s.checkSimplificationDiagnostics())
	satOK(t, s.checkSimplificationDiagnostics())
	want := uint64(0)
	if satStorageSourceDebug {
		want = 2 // One invariant check per source checkpoint.
	}
	if got := s.propagation.limit.Steps - before; got != want {
		t.Fatalf("source simplification checks consumed %d checks, want %d", got, want)
	}
}
