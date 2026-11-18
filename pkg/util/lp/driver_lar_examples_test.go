package lp

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

func lpRunLARExample(d *LARExampleDriver, term bool) error {
	if term {
		return d.RunTerm()
	}
	return d.RunEvidence()
}

func TestLPLARExamplesActualSourceProfiles(t *testing.T) {
	for _, term := range []bool{false, true} {
		h := newLPPairHarness(t)
		w := &lpDriverFlushWriter{}
		d := NewLARExampleDriver(h.m, w)
		err := lpRunLARExample(d, term)
		if sourceDebugEnabled {
			if err != ErrAssertion || d.closed || d.Solver == nil || w.Len() != 0 || d.Model != nil {
				t.Fatal(term, err, w.String())
			}
			want := uint32(2)
			if term {
				want = 0
			}
			if d.Solver.NumberOfConstraints() != want {
				t.Fatal(term, d.Solver.NumberOfConstraints())
			}
		} else {
			h.must(err)
			want := "OPTIMAL\n"
			if term {
				want += "v1000000 = 0,v1 = 0,v0 = 0,\n"
			}
			if w.String() != want || !d.closed || d.Status != StatusOptimal || d.Solver != nil || d.Model != nil {
				t.Fatal(term, w.String(), d.Status, d.closed)
			}
			if term {
				if d.Terms != 1 || d.Columns != 3 || d.Constraints != 0 || !reflect.DeepEqual(d.Solution, []DriverLARModelValue{{2, "v1000000", 0}, {1, "v1", 0}, {0, "v0", 0}}) {
					t.Fatal(d.Terms, d.Columns, d.Constraints, d.Solution)
				}
			} else if d.Terms != 0 || d.Columns != 2 || d.Constraints != 2 || len(d.Solution) != 0 {
				t.Fatal(d.Terms, d.Columns, d.Constraints, d.Solution)
			}
		}
		if lpRunLARExample(d, term) != ErrExactVectorPending {
			t.Fatal("repeat entry")
		}
		h.must(d.Close())
		h.must(d.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	for _, term := range []bool{false, true} {
		d := NewLARExampleDriver(nil, io.Discard)
		if lpRunLARExample(d, term) != ErrBounds {
			t.Fatal("nil manager")
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		if lpRunLARExample(d, term) != ErrExactVectorPending {
			t.Fatal("closed entry")
		}
	}
}

func TestLPLARExamplesReportFailurePrefixes(t *testing.T) {
	if sourceDebugEnabled {
		t.Skip("source assertions precede reports; profile covered by source test")
	}
	for _, term := range []bool{false, true} {
		message := "OPTIMAL\n"
		if term {
			message += "v1000000 = 0,v1 = 0,v0 = 0,\n"
		}
		for at := 0; at < len(message); at++ {
			h := newLPPairHarness(t)
			w := &lpResidualFailWriter{remaining: at}
			d := NewLARExampleDriver(h.m, lpResidualWriterOnly{w})
			if err := lpRunLARExample(d, term); err != lpErrResidualWrite || w.String() != message[:at] || d.closed || d.Solver == nil || d.Status != StatusOptimal {
				t.Fatal(term, at, err, w.String())
			}
			if term {
				completed := strings.Count(message[:at], ",")
				if len(d.Solution) != completed {
					t.Fatal(at, len(d.Solution), completed)
				}
				if (d.Model != nil) != (at >= len("OPTIMAL\n")) {
					t.Fatal(at, d.Model)
				}
			}
			h.must(d.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
		for at := 1; at <= strings.Count(message, "\n"); at++ {
			h := newLPPairHarness(t)
			w := &lpDriverFlushWriter{failAt: at}
			d := NewLARExampleDriver(h.m, w)
			if err := lpRunLARExample(d, term); err != lpErrResidualWrite || w.flushes != at || d.closed {
				t.Fatal(term, at, err, w)
			}
			h.must(d.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

// Source literals and all reached arithmetic fit small numeral representation.
// The actual disabled-budget run must not acquire custom Runtime storage.
func TestLPLARExamplesSmallNumeralOwnership(t *testing.T) {
	for _, term := range []bool{false, true} {
		for _, direct := range []bool{false, true} {
			var h *lpPairHarness
			if direct {
				h = lpNewDirectSparseHarness(t)
			} else {
				h = newLPPairHarness(t)
			}
			d := NewLARExampleDriver(h.m, io.Discard)
			used := h.budget.Used()
			h.budget.FailAfter(0)
			err := lpRunLARExample(d, term)
			h.budget.FailAfter(-1)
			if err != sourceAssertionFailure() || h.budget.Used() != used {
				t.Fatal(term, direct, err, h.budget.Used(), used)
			}
			if !sourceDebugEnabled && (!d.closed || d.Status != StatusOptimal) {
				t.Fatal(term, direct, d.Status, d.closed)
			}
			h.must(d.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLPLARExamplesPreserveOlderComparison(t *testing.T) {
	for _, term := range []bool{false, true} {
		found := false
		for ordinal := int64(0); ordinal < 40 && !found; ordinal++ {
			h := newLPPairHarness(t)
			v := h.pairExactLarge(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
			var caller exactEtaCalls
			before, err := caller.begin(h.m)
			h.must(err)
			h.budget.FailAfter(ordinal)
			_, err = h.m.Compare(&v.X, &v.Y)
			h.budget.FailAfter(-1)
			err = caller.after(h.m, before, err)
			older := h.m.ComparisonInvocationState()
			d := NewLARExampleDriver(h.m, io.Discard)
			if older.Active {
				found = true
				if !lpExactPriorityPhysicalRefusal(err) {
					t.Fatal(err)
				}
				_ = lpRunLARExample(d, term)
				if d.calls.comparison != 0 || d.Solver != nil && d.Solver.calls.comparison != 0 || d.Model != nil && d.Model.calls.comparison != 0 {
					t.Fatal("adopted caller comparison")
				}
			}
			h.must(d.Close())
			if older.Active && h.m.ComparisonInvocationState() != older {
				t.Fatal("consumed caller comparison")
			}
			h.must(caller.cleanupRaw(h.m))
			h.must(caller.cleanupRoots(h.m))
			lpExactVectorNamedCensus(t, h, nil)
		}
		if !found {
			t.Fatal(term, "no actual older comparison")
		}
	}
}
