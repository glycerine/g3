package lp

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func lpRequireGLPK(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("glpsol"); err != nil {
		t.Skip("actual glpsol executable is not installed")
	}
}

func lpProcessFixedGLPKModel(cost int64) string {
	text := "NAME          DRIVER\nROWS\n N  COST\n L  R\nCOLUMNS\n"
	text += lpExactMPSPair("x", "COST", fmt.Sprint(cost)) + lpExactMPSPair("x", "R", "1")
	text += lpExactMPSPair("y", "COST", "1") + lpExactMPSPair("y", "R", "1")
	text += "RHS\n" + lpExactMPSPair("RHS1", "R", "6") + "BOUNDS\n"
	text += fmt.Sprintf(" UP %-8s  %-8s  %s\n", "BND", "x", "4")
	text += fmt.Sprintf(" UP %-8s  %-8s  %s\n", "BND", "y", "5")
	return text + "ENDATA\n"
}

func TestLPProcessFileActualGLPKIndependentMinMax(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	lpRequireGLPK(t)
	for world := 0; world < 4; world++ {
		for _, exact := range []bool{false, true} {
			for _, minimum := range []bool{false, true} {
				h := newLPPairHarness(t)
				directory := filepath.Join(t.TempDir(), "literal models ; space")
				if err := os.Mkdir(directory, 0700); err != nil {
					t.Fatal(err)
				}
				name := "knapsack name.mps"
				if err := os.WriteFile(filepath.Join(directory, name), []byte(lpProcessFixedGLPKModel(int64(3+world))), 0600); err != nil {
					t.Fatal(err)
				}
				args := lpMPSDriverArguments(t, exact, false, minimum, true)
				args.usedFlags["--compare_with_glpk"] = true
				// process_test_file forwards explicit limits; it does not reparse these.
				args.setValue("--max_iters", "invalid ignored")
				args.setValue("--time_limit", "invalid ignored")
				var output bytes.Buffer
				counts := DriverComparisonCounts{Successes: 5, Failures: 7, Inconclusives: 11}
				clock := &lpMPSDriverClock{}
				d := NewProcessFileDriver(directory, name, args, t.TempDir(), 1000, 3, &counts, h.m, MPSDriverConfig{Output: &output, Clock: clock.read})
				h.must(d.Run())
				if !d.closed || d.file != nil || d.MPS != nil || d.LAR != nil || d.CaughtError != nil || d.GLPKExitCode != 0 || d.GLPKLaunchError != nil || counts != (DriverComparisonCounts{6, 7, 11}) {
					t.Fatal(world, exact, minimum, d.CaughtError, d.GLPKLaunchError, counts)
				}
				report, err := os.ReadFile(d.ReportName)
				h.must(err)
				optimum := 4*(3+world) + 2
				if minimum {
					optimum = 0
				}
				wantCost := fmt.Sprintf("cost = %d\n", optimum)
				if minimum && !exact {
					wantCost = "cost = -0\n"
				}
				if !strings.Contains(string(report), "Status: OPTIMAL\n") || !strings.Contains(string(report), wantCost) || strings.Contains(string(report), "primal cost") {
					t.Fatal(world, exact, minimum, string(report))
				}
				if output.String() != "processing "+directory+"/"+name+"\n" {
					t.Fatal(output.String())
				}
				if d.Run() != ErrExactVectorPending {
					t.Fatal("repeated source invocation")
				}
				h.must(d.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}
		}
	}
}

func TestLPProcessFileNamesSkipAndExplicitEntry(t *testing.T) {
	for _, minimum := range []bool{false, true} {
		for _, exact := range []bool{false, true} {
			direction := "max"
			if minimum {
				direction = "min"
			}
			suffix := ".out"
			if exact {
				suffix = "_mpq.out"
			}
			if got := DriverLPOutputName(minimum, "x.mps", exact); got != "x.mps_lp_tst_"+direction+suffix {
				t.Fatal(got)
			}
			if got := DriverGLPKOutputName(minimum, "x.mps"); got != "x.mps_"+direction+"_glpk_out" {
				t.Fatal(got)
			}
		}
	}
	var output bytes.Buffer
	counts := DriverComparisonCounts{1, 2, 3}
	d := NewProcessFileDriver("does not exist", "backup~", lpDriverArgs(t, "lp-test"), "also missing", 0, 0, &counts, nil, MPSDriverConfig{Output: &output})
	if err := d.Run(); err != nil || !d.closed || output.Len() != 0 || counts != (DriverComparisonCounts{1, 2, 3}) {
		t.Fatal(err, output.String(), counts)
	}
	d = NewProcessFileDriver("", "", nil, "", 0, 0, &counts, nil, MPSDriverConfig{Output: &output})
	if d.Run() != ErrBounds || d.Close() != nil {
		t.Fatal("argument guard")
	}
	d = NewProcessFileDriver("", "", lpDriverArgs(t), "", 0, 0, nil, nil, MPSDriverConfig{Output: &output})
	if d.Run() != ErrBounds || d.Close() != nil {
		t.Fatal("counter guard")
	}
	m := NewMPSFileDriver("", nil, nil, MPSDriverConfig{Output: io.Discard})
	if m.RunWithOptions(MPSDriverSolveOptions{}) != ErrBounds || m.RunWithOptions(MPSDriverSolveOptions{}) != ErrExactVectorPending {
		t.Fatal("explicit overload guards")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLPProcessFileCaughtSourceFailureAndLAR(t *testing.T) {
	h := newLPPairHarness(t)
	name := lpMPSDriverFile(t, lpMPSDriverModel(3, 1, 4, 5, 6))
	var output bytes.Buffer
	counts := DriverComparisonCounts{}
	args := lpMPSDriverArguments(t, true, true, false, false)
	args.usedFlags["--compare_with_glpk"] = true
	d := NewProcessFileDriver(filepath.Dir(name), filepath.Base(name), args, t.TempDir(), 1000, 0, &counts, h.m, MPSDriverConfig{Output: &output})
	h.must(d.Run())
	report, err := os.ReadFile(d.ReportName)
	h.must(err)
	if d.CaughtError == nil || counts.Failures != 1 || counts.Inconclusives != 0 || !strings.HasSuffix(string(report), "catching the failure\n") || d.GLPKReportName != "" || !d.closed {
		t.Fatal(d.CaughtError, counts, string(report))
	}
	h.must(d.Close())
	lpExactVectorNamedCensus(t, h, nil)
	// The actual LAR route uses its two reads and reports into the same real file.
	h = newLPPairHarness(t)
	name = lpMPSDriverFile(t, lpLARFreeMPS(2))
	args = lpDriverArgs(t, "lp-test", "--lar", "--randomize_lar")
	counts = DriverComparisonCounts{}
	output.Reset()
	clock := &lpMPSDriverClock{}
	d = NewProcessFileDriver(filepath.Dir(name), filepath.Base(name), args, t.TempDir(), 0, 0, &counts, h.m, MPSDriverConfig{Output: &output, Clock: clock.read})
	h.must(d.Run())
	report, err = os.ReadFile(d.ReportName)
	h.must(err)
	if d.CaughtError != nil || counts != (DriverComparisonCounts{}) || string(report) != "status is OPTIMAL, processed for 2 seconds, and 1 iterations\nchecking randomize\n" {
		t.Fatal(d.CaughtError, counts, string(report))
	}
	h.must(d.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPProcessFileProcessingReportRefusals(t *testing.T) {
	text := "processing missing/input.mps\n"
	for at := 0; at < len(text); at++ {
		counts := DriverComparisonCounts{}
		w := &lpResidualFailWriter{remaining: at}
		d := NewProcessFileDriver("missing", "input.mps", lpDriverArgs(t), t.TempDir(), 0, 0, &counts, nil, MPSDriverConfig{Output: lpResidualWriterOnly{w}})
		if err := d.Run(); err != lpErrResidualWrite || w.String() != text[:at] || d.file != nil || counts != (DriverComparisonCounts{}) {
			t.Fatal(at, err, w.String(), counts)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
	counts := DriverComparisonCounts{}
	w := &lpMPSDriverActionWriter{failFlush: 1}
	d := NewProcessFileDriver("missing", "input.mps", lpDriverArgs(t), t.TempDir(), 0, 0, &counts, nil, MPSDriverConfig{Output: w})
	if err := d.Run(); err == nil || d.file != nil || w.flushes != 1 {
		t.Fatal(err, w.flushes)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	// Source output-open failure occurs after the processing line, before solve.
	outDirectory := filepath.Join(t.TempDir(), "absent")
	report := "processing missing/input.mps\ncannot open file " + outDirectory + "/input.mps_lp_tst_max.out\n"
	for at := -1; at < len(report); at++ {
		var output bytes.Buffer
		var out io.Writer = &output
		var fail *lpResidualFailWriter
		if at >= 0 {
			fail = &lpResidualFailWriter{remaining: at}
			out = lpResidualWriterOnly{fail}
		}
		d = NewProcessFileDriver("missing", "input.mps", lpDriverArgs(t), outDirectory, 0, 0, &counts, nil, MPSDriverConfig{Output: out})
		err := d.Run()
		if at < 0 {
			if err != ErrDriverReport || output.String() != report {
				t.Fatal(err, output.String())
			}
		} else if err != lpErrResidualWrite || fail.String() != report[:at] {
			t.Fatal(at, err, fail.String())
		}
		if d.MPS != nil || d.file != nil || counts != (DriverComparisonCounts{}) {
			t.Fatal("open failure prefix", counts)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLPProcessFileLiveFlagsAndLimits(t *testing.T) {
	h := newLPPairHarness(t)
	name := lpMPSDriverFile(t, lpMPSDriverModel(3, 1, 4, 5, 6))
	args := lpMPSDriverArguments(t, false, false, true, true)
	counts := DriverComparisonCounts{}
	var d *ProcessFileDriver
	w := &lpMPSDriverActionWriter{action: func(string) error {
		delete(args.usedFlags, "--min")
		args.usedFlags["--mpq"] = true
		return nil
	}}
	clock := &lpMPSDriverClock{}
	d = NewProcessFileDriver(filepath.Dir(name), filepath.Base(name), args, t.TempDir(), 1000, 0, &counts, h.m, MPSDriverConfig{Output: w, Clock: clock.read})
	h.must(d.Run())
	report, err := os.ReadFile(d.ReportName)
	h.must(err)
	if !strings.HasSuffix(d.ReportName, "_min.out") || strings.Contains(string(report), "in rationals") || !strings.Contains(string(report), "cost = -0\n") || strings.Contains(string(report), "primal cost") {
		t.Fatal(d.ReportName, string(report))
	}
	h.must(d.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPDriverGLPKActualExitAndLaunchFailures(t *testing.T) {
	lpRequireGLPK(t)
	name := lpMPSDriverFile(t, "invalid MPS bytes\n")
	if code, err := RunDriverGLPK(name, filepath.Join(t.TempDir(), "bad.report"), false, 0); err != nil || code == 0 {
		t.Fatal(code, err)
	}
	// This is a real executable lookup failure, not an emulated subprocess.
	t.Setenv("PATH", t.TempDir())
	if code, err := RunDriverGLPK(name, filepath.Join(t.TempDir(), "missing.report"), true, 1); err == nil || code != -1 {
		t.Fatal(code, err)
	}
}

func TestLPProcessFileGLPKInconclusivePrefixes(t *testing.T) {
	// Solve still uses real files and algorithms; only executable lookup fails.
	t.Setenv("PATH", t.TempDir())
	name := lpMPSDriverFile(t, lpMPSDriverModel(3, 1, 4, 5, 6))
	args := lpMPSDriverArguments(t, false, false, false, false)
	args.usedFlags["--compare_with_glpk"] = true
	report := "processing " + name + "\nglpk failed\n"
	for at := -1; at < len(report); at++ {
		var output bytes.Buffer
		var out io.Writer = &output
		var fail *lpResidualFailWriter
		if at >= 0 {
			fail = &lpResidualFailWriter{remaining: at}
			out = lpResidualWriterOnly{fail}
		}
		counts := DriverComparisonCounts{}
		clock := &lpMPSDriverClock{}
		d := NewProcessFileDriver(filepath.Dir(name), filepath.Base(name), args, t.TempDir(), 1000, 0, &counts, nil, MPSDriverConfig{Output: out, Clock: clock.read})
		err := d.Run()
		if at < 0 {
			if err != nil || output.String() != report || counts.Inconclusives != 1 || d.GLPKLaunchError == nil || d.GLPKExitCode != -1 {
				t.Fatal(err, output.String(), counts)
			}
		} else if err != lpErrResidualWrite || fail.String() != report[:at] || counts.Inconclusives != 0 {
			t.Fatal(at, err, fail.String(), counts)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
	counts := DriverComparisonCounts{}
	w := &lpMPSDriverActionWriter{failFlush: 2}
	d := NewProcessFileDriver(filepath.Dir(name), filepath.Base(name), args, t.TempDir(), 1000, 0, &counts, nil, MPSDriverConfig{Output: w})
	if err := d.Run(); err == nil || w.flushes != 2 || counts.Inconclusives != 0 {
		t.Fatal(err, w.flushes, counts)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLPProcessFilePhysicalChildOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	large := strings.Repeat("1234567890", 20) + "7/13"
	input := strings.Replace(lpMPSDriverModel(3, 2, 4, 5, 3), lpExactMPSPair("x", "COST", "3"), lpExactMPSPair("x", "COST", large), 1)
	name := lpMPSDriverFile(t, input)
	for _, direct := range []bool{false, true} {
		success, failures := false, 0
		for ordinal := int64(0); ordinal < 6000 && !success; ordinal++ {
			if !t.Run(fmt.Sprintf("direct%t/%d", direct, ordinal), func(t *testing.T) {
				var h *lpPairHarness
				if direct {
					h = lpNewDirectSparseHarness(t)
				} else {
					h = newLPPairHarness(t)
				}
				counts := DriverComparisonCounts{}
				clock := &lpMPSDriverClock{}
				d := NewProcessFileDriver(filepath.Dir(name), filepath.Base(name), lpMPSDriverArguments(t, true, false, false, false), t.TempDir(), 1000, 0, &counts, h.m, MPSDriverConfig{Output: io.Discard, Clock: clock.read})
				h.budget.FailAfter(ordinal)
				err := d.Run()
				h.budget.FailAfter(-1)
				cause := err
				if d.CaughtError != nil {
					cause = d.CaughtError
				}
				if cause == nil {
					success = true
					if !d.closed || counts != (DriverComparisonCounts{}) {
						t.Fatal("completion", counts)
					}
				} else {
					failures++
					if !lpExactPriorityPhysicalRefusal(cause) || d.Run() != ErrExactVectorPending {
						t.Fatal("actual child refusal", cause, err)
					}
				}
				h.must(d.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}) {
				t.FailNow()
			}
		}
		if !success || direct && failures == 0 {
			t.Fatal("physical campaign", direct, success, failures)
		}
	}
}

func TestLPProcessFilePreservesOlderComparison(t *testing.T) {
	for _, lar := range []bool{false, true} {
		name := lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 4, 5, 3))
		if lar {
			name = lpMPSDriverFile(t, lpLARFreeMPS(2))
		}
		matched := false
		for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
			h := newLPPairHarness(t)
			args := lpMPSDriverArguments(t, true, false, false, false)
			if lar {
				args.usedFlags["--lar"] = true
			}
			counts := DriverComparisonCounts{}
			d := NewProcessFileDriver(filepath.Dir(name), filepath.Base(name), args, t.TempDir(), 1000, 0, &counts, h.m, MPSDriverConfig{Output: io.Discard})
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
				if d.MPS != nil && d.MPS.calls.comparison != 0 {
					t.Fatal("parent adopted caller comparison")
				}
			}
			h.must(d.Close())
			if older.Active && h.m.ComparisonInvocationState() != older {
				t.Fatal("cleanup consumed caller comparison")
			}
			h.must(caller.cleanupRaw(h.m))
			h.must(caller.cleanupRoots(h.m))
			lpExactVectorNamedCensus(t, h, nil)
		}
		if !matched {
			t.Fatal("missing actual older comparison", lar)
		}
	}
}

func TestLPProcessFileRealClosedReportFailure(t *testing.T) {
	name := lpMPSDriverFile(t, lpMPSDriverModel(3, 1, 4, 5, 6))
	counts := DriverComparisonCounts{}
	var d *ProcessFileDriver
	closed := false
	clock := func() (uint64, error) {
		if !closed {
			if err := d.file.Close(); err != nil {
				t.Fatal(err)
			}
			closed = true
		}
		return 1, nil
	}
	d = NewProcessFileDriver(filepath.Dir(name), filepath.Base(name), lpMPSDriverArguments(t, false, false, false, false), t.TempDir(), 1000, 0, &counts, nil, MPSDriverConfig{Output: io.Discard, Clock: clock})
	if err := d.Run(); err == nil || d.CaughtError == nil || counts.Failures != 0 || d.file == nil {
		t.Fatal(err, d.CaughtError, counts)
	}
	if err := d.Close(); err == nil {
		t.Fatal("closed descriptor must report its real close failure")
	}
	// The descriptor is already actually closed; release its retained metadata.
	d.file = nil
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLPProcessFileActualLARGLPKReportMismatch(t *testing.T) {
	lpRequireGLPK(t)
	h := newLPPairHarness(t)
	text := "NAME          FREE\nROWS\n N  COST\nCOLUMNS\n" + lpExactMPSPair("x", "COST", "1") + lpExactMPSPair("y", "COST", "2") + "RHS\nBOUNDS\n"
	text += fmt.Sprintf(" FR %-8s  %-8s\n", "BND", "x") + fmt.Sprintf(" FR %-8s  %-8s\n", "BND", "y") + "ENDATA\n"
	name := lpMPSDriverFile(t, text)
	args := lpDriverArgs(t, "lp-test", "--lar", "--compare_with_glpk")
	counts := DriverComparisonCounts{}
	var output bytes.Buffer
	clock := &lpMPSDriverClock{}
	d := NewProcessFileDriver(filepath.Dir(name), filepath.Base(name), args, t.TempDir(), 1000, 0, &counts, h.m, MPSDriverConfig{Output: &output, Clock: clock.read})
	// Actual LAR reports lowercase "status is"; source comparison searches
	// uppercase "Status" and throws before any counter is changed.
	err := d.Run()
	if err != ErrDriverReport || d.GLPKExitCode != 0 || d.CaughtError != nil || counts != (DriverComparisonCounts{}) || d.file == nil || d.closed {
		t.Fatal(err, d.GLPKExitCode, d.CaughtError, counts, output.String())
	}
	if !strings.HasSuffix(output.String(), "cannot find the status line in "+d.ReportName+"\n") {
		t.Fatal(output.String())
	}
	h.must(d.Close())
	lpExactVectorNamedCensus(t, h, nil)
}
