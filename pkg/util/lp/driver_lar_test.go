package lp

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"testing"
)

func TestLPLARDriverIndependentSolves(t *testing.T) {
	for world := 0; world < 20; world++ {
		for _, impossible := range []bool{false, true} {
			h := newLPPairHarness(t)
			s, low := lpLARFullSolveFixture(t, h, StrategyTableauRows, world, impossible, false)
			var out bytes.Buffer
			clock := &lpMPSDriverClock{}
			args := lpDriverArgs(t, "lp-test", "--maxng", "700", "--max_iters", "900", "-pd", "--randomize_lar")
			d := NewLARRunDriver(s, nil, args, MPSDriverConfig{Output: &out, Clock: clock.read})
			h.must(d.Run())
			want := StatusOptimal
			if impossible {
				want = StatusInfeasible
			}
			text, err := StatusString(want)
			h.must(err)
			if d.Status != want || !d.closed || d.evidence != nil || clock.calls != 3 || !strings.Contains(out.String(), "status is "+text+", processed for 2 seconds, and ") || s.Settings.MaxIterationsWithoutImprovements != 700 || s.Settings.MaxTotalIterations != 900 || !s.Settings.PresolveWithDoubleSolverForLAR {
				t.Fatal(world, impossible, d.Status, clock.calls, out.String())
			}
			if impossible {
				if !strings.HasSuffix(out.String(), "cannot check randomize on an infeazible  problem\n") {
					t.Fatal(out.String())
				}
			} else {
				if !strings.HasSuffix(out.String(), "checking randomize\n") {
					t.Fatal(out.String())
				}
				model := NewLARRationalModel(h.m)
				h.must(s.GetModel(model))
				v, err := h.m.String(model.Values[0])
				h.must(err)
				value, ok := new(big.Rat).SetString(v)
				if !ok || value.Cmp(low) < 0 {
					t.Fatal(v, low)
				}
				otherText, err := h.m.String(model.Values[1])
				h.must(err)
				other := mustLPRat(t, otherText)
				a := big.NewRat(int64(1+world%7), int64(1+world%5))
				b := big.NewRat(int64(1+world%11), int64(1+world%3))
				lhs := new(big.Rat).Add(new(big.Rat).Mul(a, value), new(big.Rat).Mul(b, other))
				cap := new(big.Rat).Add(new(big.Rat).Mul(a, low), big.NewRat(1, 17))
				if other.Sign() < 0 || lhs.Cmp(cap) > 0 {
					t.Fatal("randomized model violates independently calculated constraints", lhs, cap)
				}
				h.must(model.Close())
			}
			if d.Run() != ErrExactVectorPending {
				t.Fatal("driver retried")
			}
			h.must(d.Close())
			h.must(s.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func lpLARDriverFree(t *testing.T, h *lpPairHarness, count int) *LARSolver {
	t.Helper()
	s, err := NewLARSolver(h.m)
	h.must(err)
	for j := 0; j < count; j++ {
		_, err := s.AddVar(uint32(j + 1000))
		h.must(err)
	}
	return s
}

func TestLPLARDriverRandomizeHundredCap(t *testing.T) {
	h := newLPPairHarness(t)
	s := lpLARDriverFree(t, h, 110)
	d := NewLARRunDriver(s, nil, lpDriverArgs(t, "lp-test", "--randomize_lar"), MPSDriverConfig{Output: io.Discard, Clock: (&lpMPSDriverClock{}).read})
	h.must(d.Run())
	model := NewLARRationalModel(h.m)
	h.must(s.GetModel(model))
	values := make(map[string]bool)
	for j := uint32(0); j < 110; j++ {
		v, err := h.m.String(model.Values[j])
		h.must(err)
		if j < 100 {
			values[v] = true
		} else if v != "0" {
			t.Fatal("randomized beyond source cap", j, v)
		}
	}
	if len(values) != 100 {
		t.Fatal("first hundred did not randomize distinctly", len(values))
	}
	h.must(model.Close())
	h.must(s.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPLARDriverComparison(t *testing.T) {
	for _, withReader := range []bool{false, true} {
		h := newLPPairHarness(t)
		s := lpLARDriverFree(t, h, 1)
		var reader *RationalMPSReader
		if withReader {
			reader = OpenRationalMPSReader(h.m, lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 2, 4, 3)))
			reader.Messages = io.Discard
			h.must(reader.Read())
		}
		var out bytes.Buffer
		clock := &lpMPSDriverClock{failAt: 1}
		d := NewLARRunDriver(s, reader, lpDriverArgs(t, "lp-test", "--compare_with_primal"), MPSDriverConfig{Output: &out, Clock: clock.read})
		h.must(d.Run())
		want := "cannot compare with primal, the reader is null \n"
		if withReader {
			want = "extended A[1,3]\nstatus = OPTIMAL\n"
		}
		if out.String() != want || clock.calls != 0 || s.Bounds.Status.Value != StatusOptimal || !d.closed || d.values != nil || d.comparison != nil {
			t.Fatal(withReader, out.String(), clock.calls)
		}
		h.must(s.Close())
		if reader != nil {
			h.must(reader.Close())
		}
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPLARDriverClockAndOptionPrefixes(t *testing.T) {
	defaultOut := NewLARRunDriver(nil, nil, nil, MPSDriverConfig{})
	if defaultOut.out != os.Stdout {
		t.Fatal("missing native default output")
	}
	lpStaticMust(t, defaultOut.Close())
	for _, profile := range []string{"clock1", "clock2", "clock3", "bad-maxng", "bad-maxiters", "nil-args", "nil-solver", "closed"} {
		h := newLPPairHarness(t)
		s := lpLARDriverFree(t, h, 1)
		var out bytes.Buffer
		clock := &lpMPSDriverClock{}
		args := lpDriverArgs(t, "lp-test", "--maxng", "700", "--max_iters", "900")
		d := NewLARRunDriver(s, nil, args, MPSDriverConfig{Output: &out, Clock: clock.read})
		if strings.HasPrefix(profile, "clock") {
			clock.failAt = int(profile[len(profile)-1] - '0')
		} else if profile == "bad-maxng" {
			args.usedValues.Set("--maxng", "bad")
		} else if profile == "bad-maxiters" {
			args.usedValues.Set("--max_iters", "bad")
		} else if profile == "nil-args" {
			d.Arguments = nil
		} else if profile == "nil-solver" {
			d.Solver = nil
		} else {
			h.must(d.Close())
		}
		err := d.Run()
		if err == nil || strings.HasPrefix(profile, "clock") && err != lpErrMPSDriverClock {
			t.Fatal(profile, err)
		}
		if profile == "clock1" && out.Len() != 0 || (profile == "clock2" || profile == "clock3") && out.String() != "status is OPTIMAL, processed for " {
			t.Fatal(profile, out.String())
		}
		if profile == "bad-maxiters" && s.Settings.MaxIterationsWithoutImprovements != 700 {
			t.Fatal("earlier option prefix lost")
		}
		h.must(d.Close())
		h.must(s.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPLARDriverOlderComparison(t *testing.T) {
	for _, compare := range []bool{false, true} {
		matched := false
		for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
			h := newLPPairHarness(t)
			s := lpLARDriverFree(t, h, 1)
			flags := []string{"lp-test"}
			var r *RationalMPSReader
			if compare {
				r = OpenRationalMPSReader(h.m, lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 2, 4, 3)))
				r.Messages = io.Discard
				h.must(r.Read())
				flags = append(flags, "--compare_with_primal")
			}
			d := NewLARRunDriver(s, r, lpDriverArgs(t, flags...), MPSDriverConfig{Output: io.Discard, Clock: (&lpMPSDriverClock{}).read})
			v := h.pairExactLarge(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
			var caller exactEtaCalls
			before, err := caller.begin(h.m)
			h.must(err)
			h.budget.FailAfter(ordinal)
			_, err = h.m.Compare(&v.X, &v.Y)
			h.budget.FailAfter(-1)
			err = caller.after(h.m, before, err)
			older := h.m.ComparisonInvocationState()
			if older.Active {
				matched = true
				if !lpExactPriorityPhysicalRefusal(err) {
					t.Fatal(err)
				}
				_ = d.Run()
				if s.calls.comparison != 0 || d.comparison != nil && (d.comparison.outerCalls.comparison != 0 || d.comparison.calls.comparison != 0) {
					t.Fatal("LAR driver child adopted caller comparison")
				}
			}
			h.must(d.Close())
			h.must(s.Close())
			if r != nil {
				h.must(r.Close())
			}
			if older.Active && h.m.ComparisonInvocationState() != older {
				t.Fatal("driver cleanup consumed caller receipt")
			}
			h.must(caller.cleanupRaw(h.m))
			h.must(caller.cleanupRoots(h.m))
			lpExactVectorNamedCensus(t, h, nil)
		}
		if !matched {
			t.Fatal("no actual older caller comparison", compare)
		}
	}
}

func TestLPLARDriverReportFailures(t *testing.T) {
	for _, compare := range []bool{false, true} {
		want := "status is OPTIMAL, processed for 2 seconds, and 1 iterations\nchecking randomize\n"
		flags := []string{"lp-test", "--randomize_lar"}
		if compare {
			want = "cannot compare with primal, the reader is null \n"
			flags = []string{"lp-test", "--compare_with_primal"}
		}
		for accepted := 0; accepted <= len(want); accepted++ {
			h := newLPPairHarness(t)
			s := lpLARDriverFree(t, h, 1)
			w := &lpResidualFailWriter{remaining: accepted}
			d := NewLARRunDriver(s, nil, lpDriverArgs(t, flags...), MPSDriverConfig{Output: lpResidualWriterOnly{writer: w}, Clock: (&lpMPSDriverClock{}).read})
			err := d.Run()
			if w.String() != want[:accepted] || accepted < len(want) && err != lpErrResidualWrite || accepted == len(want) && err != nil {
				t.Fatal(compare, accepted, err, fmt.Sprintf("%q", w.String()))
			}
			h.must(d.Close())
			h.must(s.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLPLARDriverReportFlushAndLiveFields(t *testing.T) {
	for flush := 1; flush <= 2; flush++ {
		h := newLPPairHarness(t)
		s := lpLARDriverFree(t, h, 1)
		w := &lpMPSDriverActionWriter{failFlush: flush}
		d := NewLARRunDriver(s, nil, lpDriverArgs(t, "lp-test", "--randomize_lar"), MPSDriverConfig{Output: w, Clock: (&lpMPSDriverClock{}).read})
		if err := d.Run(); err != lpErrResidualWrite || w.flushes != flush {
			t.Fatal(flush, err, w.flushes)
		}
		h.must(d.Close())
		h.must(s.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	h := newLPPairHarness(t)
	s := lpLARDriverFree(t, h, 1)
	w := &lpMPSDriverActionWriter{}
	w.action = func(text string) error {
		if text == "status is " {
			// The printed status is Solve's local result; subsequent source
			// randomize guard reads the solver's live status independently.
			s.Bounds.Status.Value = StatusFeasible
		}
		if strings.HasSuffix(text, " seconds, and ") {
			s.Core.RPrimal.TotalIterations = 7
		}
		return nil
	}
	d := NewLARRunDriver(s, nil, lpDriverArgs(t, "lp-test", "--randomize_lar"), MPSDriverConfig{Output: w, Clock: (&lpMPSDriverClock{}).read})
	h.must(d.Run())
	if w.buffer.String() != "status is OPTIMAL, processed for 2 seconds, and 7 iterations\ncannot check randomize on an infeazible  problem\n" {
		t.Fatal(w.buffer.String())
	}
	h.must(s.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPLARDriverPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	// The comparison phase reparses this field on every fault ordinal. The
	// shorter value still exercises multi-limb reader and driver allocations.
	input := strings.Replace(lpMPSDriverModel(3, 2, 2, 4, 3), lpExactMPSPair("x", "COST", "3"), lpExactMPSPair("x", "COST", strings.Repeat("1234567890", 20)+"7/13"), 1)
	name := lpMPSDriverFile(t, input)
	for _, phase := range []string{"solve", "infeasible", "randomize", "compare"} {
		compare := phase == "compare"
		for _, direct := range []bool{false, true} {
			success, failures := false, 0
			for ordinal := int64(0); ordinal < 6000 && !success; ordinal++ {
				if !t.Run(fmt.Sprintf("%s/direct%t/%d", phase, direct, ordinal), func(t *testing.T) {
					var h *lpPairHarness
					if direct {
						h = lpNewDirectSparseHarness(t)
					} else {
						h = newLPPairHarness(t)
					}
					var s *LARSolver
					var reader *RationalMPSReader
					flags := []string{"lp-test"}
					if compare {
						s = lpLARDriverFree(t, h, 1)
						reader = OpenRationalMPSReader(h.m, name)
						reader.Messages = io.Discard
						h.must(reader.Read())
						flags = append(flags, "--compare_with_primal")
					} else {
						s, _ = lpLARFullSolveNumericFixture(t, h, StrategyTableauRows, 0, phase == "infeasible", false, true)
						if phase == "randomize" {
							flags = append(flags, "--randomize_lar")
						}
					}
					d := NewLARRunDriver(s, reader, lpDriverArgs(t, flags...), MPSDriverConfig{Output: io.Discard, Clock: (&lpMPSDriverClock{}).read})
					h.budget.FailAfter(ordinal)
					err := d.Run()
					h.budget.FailAfter(-1)
					if err == nil {
						success = true
						want := StatusOptimal
						if phase == "infeasible" {
							want = StatusInfeasible
						}
						if !d.closed || d.Status != want {
							t.Fatal(d.Status, d.closed)
						}
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
					}
					h.must(d.Close())
					h.must(s.Close())
					if reader != nil {
						h.must(reader.Close())
					}
					lpExactVectorNamedCensus(t, h, nil)
				}) {
					t.FailNow()
				}
			}
			if !success || direct && failures == 0 {
				t.Fatal(compare, direct, success, failures)
			}
		}
	}
}

func TestLPLARDriverActualComparisonReportAndLookupFailures(t *testing.T) {
	name := lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 2, 4, 3))
	want := "extended A[1,3]\nstatus = OPTIMAL\n"
	for accepted := 0; accepted <= len(want); accepted++ {
		h := newLPPairHarness(t)
		s := lpLARDriverFree(t, h, 1)
		r := OpenRationalMPSReader(h.m, name)
		r.Messages = io.Discard
		h.must(r.Read())
		w := &lpResidualFailWriter{remaining: accepted}
		d := NewLARRunDriver(s, r, lpDriverArgs(t, "lp-test", "--compare_with_primal"), MPSDriverConfig{Output: lpResidualWriterOnly{writer: w}})
		err := d.Run()
		if w.String() != want[:accepted] || accepted < len(want) && err != lpErrResidualWrite || accepted == len(want) && err != nil {
			t.Fatal(accepted, err, w.String())
		}
		h.must(d.Close())
		h.must(s.Close())
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	h := newLPPairHarness(t)
	s := lpLARDriverFree(t, h, 1)
	r := OpenRationalMPSReader(h.m, name)
	r.Messages = io.Discard
	h.must(r.Read())
	w := &lpMPSDriverActionWriter{}
	var d *LARRunDriver
	w.action = func(text string) error {
		if strings.HasPrefix(text, "extended A") {
			d.comparison.NamesToColumns.Del("x")
		}
		return nil
	}
	d = NewLARRunDriver(s, r, lpDriverArgs(t, "lp-test", "--compare_with_primal"), MPSDriverConfig{Output: w})
	if err := d.Run(); err == nil || err.Error() != "get_column_value_by_name x" || d.values == nil || strings.Contains(w.buffer.String(), "status = ") {
		t.Fatal(err, w.buffer.String())
	}
	h.must(d.Close())
	h.must(s.Close())
	h.must(r.Close())
	lpExactVectorNamedCensus(t, h, nil)
}
