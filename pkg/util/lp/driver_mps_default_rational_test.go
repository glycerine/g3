package lp

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestLPMPSDefaultRationalSourceDefaults(t *testing.T) {
	for world := 0; world < 40; world++ {
		h := newLPPairHarness(t)
		name := lpMPSDriverFile(t, lpMPSDriverModel(int64(2+world), int64(1+world%7), 2, 4, 3))
		var out bytes.Buffer
		clock := &lpMPSDriverClock{failAt: 1}
		// This source consumer deliberately ignores all parser settings,
		// including malformed values which the setup-based driver rejects.
		args := lpDriverArgs(t, "lp-test", "--min", "--mpq", "--max_iters", "bad", "--simplex_strategy", "bad")
		d := NewMPSFileDriver(name, args, h.m, MPSDriverConfig{Output: &out, Clock: clock.read})
		h.must(d.RunRationalDefault(false))
		// print_x walks m_columns in libstdc++ order: y, x.
		want := "solving " + name + "\nextended A[1,3]\nstatus is OPTIMAL\ny=0 x=0 \ncost = 0\n"
		if out.String() != want || d.Status != StatusOptimal || d.TotalIterations != 1 || clock.calls != 0 || d.minimum || !d.closed || d.work != nil {
			t.Fatal(world, out.String(), d.Status, d.TotalIterations, clock.calls)
		}
		// The default tableau retains fresh zero reduced costs. The true
		// mathematical optimum is positive; this is a source-default receipt,
		// not a claim that the auxiliary consumer solves these models correctly.
		if d.RunRationalDefault(true) != ErrExactVectorPending || d.Run() != ErrExactVectorPending {
			t.Fatal("default driver retried")
		}
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPMPSDefaultRationalEmptyInfeasibleAndDual(t *testing.T) {
	for _, profile := range []string{"empty", "infeasible", "dual-source", "missing", "nil-manager", "closed"} {
		for _, dual := range []bool{false, true} {
			h := newLPPairHarness(t)
			input := lpMPSDriverModel(3, 2, 2, 4, 3)
			if profile == "empty" {
				input = "NAME EMPTY\nROWS\n N COST\nCOLUMNS\nRHS\nENDATA\n"
			} else if profile == "infeasible" {
				input = strings.Replace(input, "ENDATA", lpMPSBoundLine("LO", "x 7")+"ENDATA", 1)
			}
			name := lpMPSDriverFile(t, input)
			if profile == "missing" {
				name += ".missing"
			}
			var out bytes.Buffer
			d := NewMPSFileDriver(name, nil, h.m, MPSDriverConfig{Output: &out})
			if profile == "nil-manager" {
				d.manager = nil
			} else if profile == "closed" {
				h.must(d.Close())
			}
			err := d.RunRationalDefault(dual)
			switch profile {
			case "empty", "infeasible":
				h.must(err)
				want := "EMPTY"
				if profile == "infeasible" {
					want = "INFEASIBLE"
				}
				if !strings.Contains(out.String(), "status is "+want+"\n") || strings.Contains(out.String(), "cost = ") {
					t.Fatal(profile, dual, out.String())
				}
			case "dual-source":
				if dual {
					want := ErrBounds
					if sourceDebugEnabled {
						want = ErrAssertion
					}
					if err != want || d.rationalDual == nil || strings.Contains(out.String(), "status is ") {
						t.Fatal(profile, dual, err, out.String())
					}
				} else {
					h.must(err)
				}
			case "missing":
				h.must(err)
				if !strings.Contains(out.String(), "cannot process "+name+"\n") {
					t.Fatal(out.String())
				}
			case "nil-manager":
				if err != ErrExactVectorOwner {
					t.Fatal(err)
				}
			case "closed":
				if err != ErrExactVectorPending || out.Len() != 0 {
					t.Fatal(err, out.String())
				}
			}
			h.must(d.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLPMPSDefaultRationalReportFailures(t *testing.T) {
	name := lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 2, 4, 3))
	h := newLPPairHarness(t)
	w := &lpMPSDriverActionWriter{}
	d := NewMPSFileDriver(name, nil, h.m, MPSDriverConfig{Output: w})
	h.must(d.RunRationalDefault(false))
	want, flushes := w.buffer.String(), w.flushes
	lpExactVectorNamedCensus(t, h, nil)
	for accepted := 0; accepted <= len(want); accepted++ {
		h := newLPPairHarness(t)
		w := &lpResidualFailWriter{remaining: accepted}
		d := NewMPSFileDriver(name, nil, h.m, MPSDriverConfig{Output: lpResidualWriterOnly{writer: w}})
		err := d.RunRationalDefault(false)
		if w.String() != want[:accepted] || (accepted < len(want) && err != lpErrResidualWrite) || (accepted == len(want) && err != nil) {
			t.Fatal(accepted, err, w.String())
		}
		h.must(d.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	for flush := 1; flush <= flushes; flush++ {
		h := newLPPairHarness(t)
		w := &lpMPSDriverActionWriter{failFlush: flush}
		d := NewMPSFileDriver(name, nil, h.m, MPSDriverConfig{Output: w})
		if err := d.RunRationalDefault(false); err != lpErrResidualWrite || w.flushes != flush || !strings.HasPrefix(want, w.buffer.String()) {
			t.Fatal(flush, err, w.buffer.String())
		}
		h.must(d.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPMPSDefaultRationalNameThresholdAndLiveStatus(t *testing.T) {
	for _, count := range []int{19, 20} {
		input := "NAME NAMES\nROWS\n N COST\n L R\nCOLUMNS\n"
		for j := 0; j < count; j++ {
			input += lpExactMPSPair(fmt.Sprintf("x%d", j), "COST", "1") + lpExactMPSPair(fmt.Sprintf("x%d", j), "R", "1")
		}
		input += "RHS\n" + lpExactMPSPair("RHS1", "R", "100") + "ENDATA\n"
		h := newLPPairHarness(t)
		var out bytes.Buffer
		d := NewMPSFileDriver(lpMPSDriverFile(t, input), nil, h.m, MPSDriverConfig{Output: &out})
		h.must(d.RunRationalDefault(false))
		if strings.Contains(out.String(), "x0=") != (count < 20) || !strings.Contains(out.String(), "\ncost = 0\n") {
			t.Fatal(count, out.String())
		}
		lpExactVectorNamedCensus(t, h, nil)
	}
	h := newLPPairHarness(t)
	w := &lpMPSDriverActionWriter{}
	var d *MPSFileDriver
	w.action = func(text string) error {
		if text == "status is " {
			d.rationalPrimal.Status = StatusFeasible
		}
		return nil
	}
	d = NewMPSFileDriver(lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 2, 4, 3)), nil, h.m, MPSDriverConfig{Output: w})
	h.must(d.RunRationalDefault(false))
	if d.Status != StatusFeasible || !strings.HasSuffix(w.buffer.String(), "status is FEASIBLE\n") || strings.Contains(w.buffer.String(), "cost = ") {
		t.Fatal(d.Status, w.buffer.String())
	}
	lpExactVectorNamedCensus(t, h, nil)
}

// The private number reporter is also exercised with the actual concrete dual
// accessor through an empty model; this is not credited as full dual solving.
func TestLPMPSDefaultRationalPreparedDualCost(t *testing.T) {
	h := newLPPairHarness(t)
	d := NewMPSFileDriver("prepared", nil, h.m, MPSDriverConfig{Output: io.Discard})
	var err error
	d.rationalDual, err = NewRationalDualSimplex(h.m)
	h.must(err)
	d.dual = true
	h.must(d.reportDefaultRationalNumber(nil))
	name := "missing"
	if err := d.reportDefaultRationalNumber(&name); err == nil || err.Error() != "get_column_value_by_name missing" {
		t.Fatal(err)
	}
	h.must(d.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPMPSDefaultRationalNumberPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, dual := range []bool{false, true} {
		for _, named := range []bool{false, true} {
			for _, direct := range []bool{false, true} {
				success, failures := false, 0
				for ordinal := int64(0); ordinal < 3000 && !success; ordinal++ {
					if !t.Run(fmt.Sprintf("dual%t/named%t/direct%t/%d", dual, named, direct, ordinal), func(t *testing.T) {
						var h *lpPairHarness
						if direct {
							h = lpNewDirectSparseHarness(t)
						} else {
							h = newLPPairHarness(t)
						}
						var out bytes.Buffer
						d := lpMPSDriverFixedObjective(t, h, dual, false, &out)
						d.rationalBase().NamesToColumns.Put("x", 0)
						var name *string
						x := "x"
						if named {
							name = &x
						}
						h.budget.FailAfter(ordinal)
						err := d.reportDefaultRationalNumber(name)
						h.budget.FailAfter(-1)
						if err == nil {
							success = true
							want := "inf\n"
							if named {
								want = "inf "
							}
							if out.String() != want || d.work != nil {
								t.Fatal(out.String(), d.work)
							}
						} else {
							failures++
							if !lpExactPriorityPhysicalRefusal(err) || d.work == nil {
								t.Fatal(err, d.work)
							}
						}
						h.must(d.Close())
						lpExactVectorNamedCensus(t, h, nil)
					}) {
						t.FailNow()
					}
				}
				if !success || direct && failures == 0 {
					t.Fatal(dual, named, direct, success, failures)
				}
			}
		}
	}
}

func TestLPMPSDefaultRationalLiveNumberLifetime(t *testing.T) {
	h := newLPPairHarness(t)
	w := &lpMPSDriverActionWriter{}
	var d *MPSFileDriver
	changed := 0
	w.action = func(text string) error {
		switch text {
		case "x=":
			changed++
			return h.m.SetInt32(&d.rationalPrimal.X.Data[0], 1)
		case "1 ":
			if d.work == nil || !d.work.Data[0].IsSmall() || d.work.Data[0].Num.Val != 1 {
				t.Fatal("named temporary did not survive insertion")
			}
			changed++
			return h.m.SetInt32(&d.rationalPrimal.X.Data[0], 2)
		case "y=":
			if d.work != nil {
				t.Fatal("previous named temporary survived its expression")
			}
		case "cost = ":
			if d.work != nil {
				t.Fatal("named temporary survived into objective expression")
			}
			changed++
			c := d.rationalPrimal.Columns.Get(0)
			return h.m.SetInt32(&c.Cost, 5)
		case "10\n":
			if d.work == nil || !d.work.Data[0].IsSmall() || d.work.Data[0].Num.Val != 10 {
				t.Fatal("objective temporary did not survive endl insertion")
			}
			changed++
			c := d.rationalPrimal.Columns.Get(0)
			return h.m.SetInt32(&c.Cost, 7)
		}
		return nil
	}
	d = NewMPSFileDriver(lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 2, 4, 3)), nil, h.m, MPSDriverConfig{Output: w})
	h.must(d.RunRationalDefault(false))
	if changed != 4 || !strings.HasSuffix(w.buffer.String(), "y=0 x=1 \ncost = 10\n") || d.work != nil {
		t.Fatal(changed, w.buffer.String())
	}
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPMPSDefaultRationalOlderComparison(t *testing.T) {
	for _, report := range []bool{false, true} {
		matched := false
		for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
			h := newLPPairHarness(t)
			d := NewMPSFileDriver(lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 2, 4, 3)), nil, h.m, MPSDriverConfig{Output: io.Discard})
			if report {
				d = lpMPSDriverFixedObjective(t, h, false, false, io.Discard)
			}
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
				if report {
					_ = d.reportDefaultRationalNumber(nil)
				} else {
					_ = d.RunRationalDefault(false)
				}
				if d.calls.comparison != 0 || d.rationalPrimal != nil && (d.rationalPrimal.outerCalls.comparison != 0 || d.rationalPrimal.calls.comparison != 0) || d.rationalReader != nil && d.rationalReader.calls.comparison != 0 {
					t.Fatal("default driver adopted older caller comparison")
				}
			}
			h.must(d.Close())
			if older.Active && h.m.ComparisonInvocationState() != older {
				t.Fatal("default cleanup consumed caller comparison")
			}
			h.must(caller.cleanupRaw(h.m))
			h.must(caller.cleanupRoots(h.m))
			lpExactVectorNamedCensus(t, h, nil)
		}
		if !matched {
			t.Fatal("no actual older comparison", report)
		}
	}
}
