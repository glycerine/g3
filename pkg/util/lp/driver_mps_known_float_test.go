package lp

import (
	"bytes"
	"io"
	"math"
	"strings"
	"testing"
)

func TestLPMPSKnownFloatSourceDefaults(t *testing.T) {
	for world := 0; world < 40; world++ {
		for _, dual := range []bool{false, true} {
			name := lpMPSDriverFile(t, lpMPSDriverModel(int64(world+2), int64(world%7+1), 2, 4, 3))
			var out bytes.Buffer
			clock := &lpMPSDriverClock{failAt: 1}
			d := NewMPSFileDriver(name, lpDriverArgs(t, "lp-test", "--min", "--mpq", "--max_iters", "bad", "--simplex_strategy", "bad"), nil, MPSDriverConfig{Output: &out, Clock: clock.read})
			err := d.RunFloatKnownSolution(map[string]float64{"x": 0, "y": 0}, StatusOptimal, dual)
			want := error(ErrBounds)
			if sourceDebugEnabled && !dual {
				want = ErrAssertion
			}
			if err != want || d.closed || d.floatReader == nil || strings.Contains(out.String(), "status is ") || clock.calls != 0 {
				t.Fatal(world, dual, err, out.String())
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			if d.RunFloatKnownSolution(nil, StatusOptimal, dual) != ErrExactVectorPending || d.Run() != ErrExactVectorPending {
				t.Fatal("retried")
			}
		}
	}
}

func TestLPMPSKnownFloatMismatchDomains(t *testing.T) {
	for _, dual := range []bool{false, true} {
		for _, profile := range []string{"wrong-status", "wrong-value", "equal-epsilon", "below-epsilon", "nan", "positive-inf", "negative-inf", "unknown-name", "nil-map", "empty-map"} {
			name := lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 2, 4, 3))
			var out bytes.Buffer
			d := lpKnownFloatPrepared(t, &out, dual)
			_ = name
			sol := map[string]float64{"x": 1}
			status := StatusOptimal
			switch profile {
			case "wrong-status":
				status = StatusInfeasible
			case "equal-epsilon":
				sol["x"] = 0.000001
			case "below-epsilon":
				sol["x"] = math.Nextafter(0.000001, 0)
			case "nan":
				sol["x"] = math.NaN()
			case "positive-inf":
				sol["x"] = math.Inf(1)
			case "negative-inf":
				sol["x"] = math.Inf(-1)
			case "unknown-name":
				sol = map[string]float64{"missing": 0}
			case "nil-map":
				sol = nil
			case "empty-map":
				sol = map[string]float64{}
			}
			err := d.reportFloatKnownSolution(sol, status)
			switch profile {
			case "wrong-status":
				want := error(ErrDriverStatusWrong)
				if sourceDebugEnabled {
					want = ErrAssertion
				}
				if err != want || !strings.HasSuffix(out.String(), "status should be INFEASIBLE\n") || d.closed {
					t.Fatal(profile, dual, err, out.String())
				}
			case "unknown-name":
				if err == nil || err.Error() != "get_column_value_by_name missing" {
					t.Fatal(profile, err)
				}
			default:
				mismatch := profile == "wrong-value" || profile == "equal-epsilon" || profile == "nan" || profile == "positive-inf" || profile == "negative-inf"
				if mismatch && sourceDebugEnabled {
					if err != ErrAssertion || d.closed {
						t.Fatal(profile, err)
					}
				} else if err != nil || !d.closed {
					t.Fatal(profile, err)
				}
				diagnostic := mismatch && profile != "nan"
				if strings.Contains(out.String(), "expected:") != diagnostic {
					t.Fatal(profile, out.String())
				}
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestLPMPSKnownFloatMissingEmptyInfeasibleAndThreshold(t *testing.T) {
	for _, dual := range []bool{false, true} {
		for _, profile := range []string{"missing", "empty", "infeasible", "19", "20", "closed"} {
			input := lpMPSDriverModel(3, 2, 2, 4, 3)
			expected := StatusOptimal
			switch profile {
			case "empty":
				input = "NAME EMPTY\nROWS\n N COST\nCOLUMNS\nRHS\nENDATA\n"
				expected = StatusEmpty
			case "infeasible":
				input = strings.Replace(input, "ENDATA", lpMPSBoundLine("LO", "x 7")+"ENDATA", 1)
				expected = StatusInfeasible
			case "19":
				input = lpLARFreeMPS(19)
				expected = StatusEmpty
			case "20":
				input = lpLARFreeMPS(20)
				expected = StatusEmpty
			}
			name := lpMPSDriverFile(t, input)
			if profile == "missing" {
				name += ".missing"
			}
			var out bytes.Buffer
			d := NewMPSFileDriver(name, nil, nil, MPSDriverConfig{Output: &out})
			if profile == "closed" {
				if err := d.Close(); err != nil {
					t.Fatal(err)
				}
			}
			err := d.RunFloatKnownSolution(nil, expected, dual)
			if profile == "closed" {
				if err != ErrExactVectorPending || out.Len() != 0 {
					t.Fatal(err, out.String())
				}
				continue
			}
			if err != nil {
				t.Fatal(profile, dual, err, out.String())
			}
			if !d.closed {
				t.Fatal("open")
			}
			if (profile == "19" || profile == "20") && strings.Contains(out.String(), "x0=") {
				t.Fatal(profile, out.String())
			}
			if (profile == "empty" || profile == "infeasible") && strings.Contains(out.String(), "cost = ") {
				t.Fatal(out.String())
			}
			if profile == "missing" && out.String() != "solving "+name+"\nsetting m_is_OK to false\ncannot process "+name+"\n" {
				t.Fatal(out.String())
			}
		}
	}
}

func TestLPMPSKnownFloatReportFailures(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, profile := range []string{"ordinary", "wrong-status", "wrong-value", "missing"} {
		name := lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 2, 4, 3))
		if profile == "missing" {
			name += ".missing"
		}
		status := StatusOptimal
		var sol map[string]float64
		if profile == "wrong-status" {
			status = StatusInfeasible
		}
		if profile == "wrong-value" {
			sol = map[string]float64{"x": 1}
		}
		w := &lpMPSDriverActionWriter{}
		d := lpKnownFloatPrepared(t, w, false)
		run := func(d *MPSFileDriver) error { return d.reportFloatKnownSolution(sol, status) }
		if profile == "missing" {
			d.Close()
			d = NewMPSFileDriver(name, nil, nil, MPSDriverConfig{Output: w})
			run = func(d *MPSFileDriver) error { return d.RunFloatKnownSolution(sol, status, false) }
		}
		terminal := run(d)
		want, flushes := w.buffer.String(), w.flushes
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		for accepted := 0; accepted <= len(want); accepted++ {
			w := &lpResidualFailWriter{remaining: accepted}
			d := lpKnownFloatPrepared(t, lpResidualWriterOnly{writer: w}, false)
			if profile == "missing" {
				d.Close()
				d = NewMPSFileDriver(name, nil, nil, MPSDriverConfig{Output: lpResidualWriterOnly{writer: w}})
			}
			err := run(d)
			expected := terminal
			if accepted < len(want) {
				expected = lpErrResidualWrite
			}
			if err != expected || w.String() != want[:accepted] {
				t.Fatal(profile, accepted, err, w.String())
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
		}
		for flush := 1; flush <= flushes; flush++ {
			w := &lpMPSDriverActionWriter{failFlush: flush}
			d := lpKnownFloatPrepared(t, w, false)
			if profile == "missing" {
				d.Close()
				d = NewMPSFileDriver(name, nil, nil, MPSDriverConfig{Output: w})
			}
			err := run(d)
			if err != lpErrResidualWrite || w.flushes != flush {
				t.Fatal(profile, flush, err)
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestLPMPSKnownFloatLiveReadsAndCleanup(t *testing.T) {
	for _, profile := range []string{"status", "assert-status", "value-reread", "nan-reread", "missing-getter", "cost", "close"} {
		name := lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 2, 4, 3))
		w := &lpMPSDriverActionWriter{}
		var d *MPSFileDriver
		sol := map[string]float64{"x": 1}
		expected := StatusOptimal
		if profile == "assert-status" {
			expected = StatusInfeasible
		}
		if profile == "nan-reread" {
			sol["x"] = math.NaN()
		}
		w.action = func(text string) error {
			if text == "status is " {
				switch profile {
				case "status":
					d.floatBase().Status = StatusInfeasible
				case "close":
					return d.floatReader.file.Close()
				}
			}
			if text == "status should be " && profile == "assert-status" {
				d.floatBase().Status = StatusInfeasible
			}
			if strings.HasPrefix(text, "expected:x=") {
				if profile == "value-reread" {
					sol["x"] = 999
					d.floatBase().GetOrCreateColumnInfo(0).SetFixedValue(1)
				}
				if profile == "missing-getter" {
					d.floatBase().NamesToColumns.Del("x")
				}
			}
			if text == "cost = " && profile == "cost" {
				d.floatBase().GetOrCreateColumnInfo(0).Cost = 5
				d.floatBase().GetOrCreateColumnInfo(0).SetFixedValue(2)
			}
			return nil
		}
		d = lpKnownFloatPrepared(t, w, false)
		_ = name
		err := d.reportFloatKnownSolution(sol, expected)
		switch profile {
		case "status":
			want := error(ErrDriverStatusWrong)
			if sourceDebugEnabled {
				want = ErrAssertion
			}
			if err != want || !strings.Contains(w.buffer.String(), "status is INFEASIBLE") {
				t.Fatal(profile, err, w.buffer.String())
			}
		case "assert-status":
			if err != ErrDriverStatusWrong {
				t.Fatal(profile, err)
			}
		case "value-reread":
			if err != nil || !strings.Contains(w.buffer.String(), "expected:x=1, got 1\n") {
				t.Fatal(profile, err, w.buffer.String())
			}
		case "missing-getter":
			if err == nil || err.Error() != "get_column_value_by_name x" {
				t.Fatal(profile, err)
			}
		case "nan-reread":
			if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
				t.Fatal(profile, err)
			}
		case "cost":
			if !strings.Contains(w.buffer.String(), "cost = 10\n") {
				t.Fatal(profile, err, w.buffer.String())
			}
		case "close":
			if sourceDebugEnabled {
				if err != ErrAssertion {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "closed") {
				t.Fatal(profile, err)
			} // earlier value mismatch may stop checked before close
		}
		err = d.Close()
		if profile == "close" && err != nil { // actual file was already closed; retry releases its owner
			if !strings.Contains(err.Error(), "closed") {
				t.Fatal(err)
			}
			err = d.Close()
		}
		if err != nil {
			t.Fatal(profile, err)
		}
	}
}

// Prepared reporting fixture: actual LU setup/solve followed by explicit fixed
// values for accessor scenarios. This preparation is deliberately separate
// from RunFloatKnownSolution, which preserves source constructor defaults.
func lpKnownFloatPrepared(t *testing.T, out io.Writer, dual bool) *MPSFileDriver {
	t.Helper()
	d := NewMPSFileDriver("", nil, nil, MPSDriverConfig{Output: out})
	d.dual = dual
	d.floatReader = OpenFloatMPSReader(lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 2, 4, 3)))
	lpStaticMust(t, d.floatReader.Read())
	var err error
	if dual {
		d.floatDual, err = d.floatReader.CreateDualSolver()
	} else {
		d.floatPrimal, err = d.floatReader.CreatePrimalSolver()
	}
	lpStaticMust(t, err)
	lpStaticMust(t, d.floatBase().SetupFromDriver(1000, 0, false, lpDriverArgs(t, "lp-test", "--rep_frq", "0"), nil))
	d.floatBase().Settings.MessageOut = nil
	d.floatBase().Settings.DebugOut = nil
	if dual {
		err = d.floatDual.FindMaximalSolution()
	} else {
		err = d.floatPrimal.FindMaximalSolution()
	}
	lpStaticMust(t, err)
	d.floatBase().GetOrCreateColumnInfo(0).SetFixedValue(0)
	d.floatBase().GetOrCreateColumnInfo(1).SetFixedValue(0)
	return d
}

func TestLPMPSKnownFloatPreparedReportThresholdAndGuards(t *testing.T) {
	for _, dual := range []bool{false, true} {
		for _, count := range []int{19, 20} {
			var out bytes.Buffer
			d := NewMPSFileDriver("", nil, nil, MPSDriverConfig{Output: &out})
			d.dual = dual
			d.floatReader = lpMPSRead(t, lpLARFreeMPS(count))
			var err error
			if dual {
				d.floatDual, err = d.floatReader.CreateDualSolver()
			} else {
				d.floatPrimal, err = d.floatReader.CreatePrimalSolver()
			}
			lpStaticMust(t, err)
			// This prepared reporter fixture skips source default Find, which returns
			// EMPTY for row-free input. Fixed columns make real getters well-defined.
			d.floatBase().Status = StatusOptimal
			for j := 0; j < count; j++ {
				d.floatBase().GetOrCreateColumnInfo(uint32(j)).SetFixedValue(0)
			}
			lpStaticMust(t, d.reportFloatKnownSolution(nil, StatusOptimal))
			if strings.Contains(out.String(), "x18=0 ") != (count < 20) {
				t.Fatal(dual, count, out.String())
			}
		}
	}
	for _, profile := range []string{"invalid-actual", "invalid-expected", "cost-getter", "nonoptimal", "printed-getter", "assert-reread"} {
		w := &lpMPSDriverActionWriter{}
		d := lpKnownFloatPrepared(t, w, false)
		expected := StatusOptimal
		var sol map[string]float64
		switch profile {
		case "invalid-actual":
			d.floatBase().Status = Status(999)
		case "invalid-expected":
			expected = Status(999)
		case "nonoptimal":
			d.floatBase().Status = StatusInfeasible
			expected = StatusInfeasible
		case "assert-reread":
			sol = map[string]float64{"x": 1}
		}
		oldColumn := d.floatBase().Columns[0]
		diagnosticStarted := false
		w.action = func(text string) error {
			if strings.HasPrefix(text, "expected:") {
				diagnosticStarted = true
			}
			if profile == "cost-getter" && text == "cost = " {
				d.floatBase().Columns[0] = nil
			}
			if profile == "printed-getter" && text == "x=" {
				d.floatBase().NamesToColumns.Del("x")
			}
			if profile == "assert-reread" && diagnosticStarted && text == "0\n" {
				d.floatBase().NamesToColumns.Del("x")
			}
			return nil
		}
		err := d.reportFloatKnownSolution(sol, expected)
		switch profile {
		case "nonoptimal":
			if err != nil || strings.Contains(w.buffer.String(), "cost") {
				t.Fatal(profile, err, w.buffer.String())
			}
		default:
			if err == nil {
				t.Fatal(profile, "no failure", w.buffer.String())
			}
		}
		// Metadata corrupted only to reach real getter/report failure. Restore the
		// column before cleanup, which owns the solver independently of reporting.
		if profile == "cost-getter" {
			d.floatBase().Columns[0] = oldColumn
		}
		lpStaticMust(t, d.Close())
	}
}
