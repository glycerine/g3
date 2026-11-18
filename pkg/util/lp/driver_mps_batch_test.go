package lp

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func lpBatchPut(t *testing.T, home, relative, text string) string {
	t.Helper()
	name := home + "/projects/lean/src/tests/util/lp/" + relative
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return name
}
func lpBatchSolveNames(report string) []string {
	var names []string
	for _, line := range strings.Split(report, "\n") {
		if strings.HasPrefix(line, "solving ") {
			names = append(names, strings.TrimSuffix(strings.TrimPrefix(line, "solving "), " in rationals "))
		}
	}
	return names
}

func TestLPMPSCatalogueLiteralOrderAndMinimumPrefix(t *testing.T) {
	prefix := "/chosen home/projects/lean/src/tests/util/lp/"
	files, minimums, err := FillDriverMPSCatalogue("/chosen home", true, io.Discard)
	if err != nil || len(files) != 108 || len(minimums) != 13 {
		t.Fatal(err, len(files), len(minimums))
	}
	first := []string{"l0redund.mps", "l1.mps", "l2.mps", "l3.mps", "l4.mps", "l4fix.mps", "plan.mps", "samp2.mps", "murtagh.mps", "l0.mps", "test_files/netlib/AFIRO.SIF", "test_files/netlib/SC50B.SIF", "test_files/netlib/SC50A.SIF", "test_files/netlib/KB2.SIF"}
	for i, relative := range first {
		if files[i] != prefix+relative {
			t.Fatal(i, files[i])
		}
	}
	if files[107] != prefix+"test_files/netlib/MAROS-R7.SIF" || files[106] != prefix+"test_files/netlib/FIT2D.SIF" {
		t.Fatal(files[106:])
	}
	seen := map[string]bool{}
	for _, name := range files {
		if seen[name] || minimums[name] {
			t.Fatal("duplicate or repaired source HOME bug", name)
		}
		seen[name] = true
	}
	rootFiles, rootMinimums, err := FillDriverMPSCatalogue("", true, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, name := range rootFiles {
		if rootMinimums[name] {
			matches++
		}
	}
	if matches != 13 {
		t.Fatal("empty but present HOME must match unprefixed minimums", matches)
	}
	// Each result owns its Go slice/map; callers cannot change the catalogue.
	files[0] = "changed"
	delete(minimums, "/projects/lean/src/tests/util/lp/test_files/netlib/FIT2P.SIF")
	again, mins, err := FillDriverMPSCatalogue("/chosen home", true, nil)
	if err != nil || again[0] != prefix+first[0] || len(mins) != 13 {
		t.Fatal(err, again[0], len(mins))
	}
}

func TestLPMPSCatalogueMissingHomeReport(t *testing.T) {
	message := "cannot find home directory, don't know how to find the files"
	for at := -1; at < len(message); at++ {
		var out bytes.Buffer
		var writer io.Writer = &out
		var fail *lpResidualFailWriter
		if at >= 0 {
			fail = &lpResidualFailWriter{remaining: at}
			writer = lpResidualWriterOnly{fail}
		}
		files, mins, err := FillDriverMPSCatalogue("ignored", false, writer)
		if files != nil || mins != nil {
			t.Fatal(files, mins)
		}
		if at < 0 {
			if err != nil || out.String() != message {
				t.Fatal(err, out.String())
			}
		} else if err != lpErrResidualWrite || fail.String() != message[:at] {
			t.Fatal(at, err, fail.String())
		}
	}
	w := &lpMPSDriverActionWriter{}
	if _, _, err := FillDriverMPSCatalogue("", false, w); err != nil || w.flushes != 0 {
		t.Fatal(err, w.flushes)
	}
}

func TestLPMPSBatchFloatPreludeAndIndependentLoopSolve(t *testing.T) {
	home := t.TempDir()
	// samp2 is absent from the nine-call prelude and solved in the final loop.
	actual := lpBatchPut(t, home, "samp2.mps", lpProcessFixedGLPKModel(3))
	args := lpMPSDriverArguments(t, false, false, true, false)
	clock := &lpMPSDriverClock{}
	var output bytes.Buffer
	d := NewMPSBatchDriver(args, nil, MPSDriverConfig{Output: &output, Clock: clock.read})
	if err := d.RunWithHome(home, true); err != nil {
		t.Fatal(err)
	}
	if !d.closed || d.Current != nil || d.Completed != 108 || d.PreludeCompleted != 9 || d.Counts != (DriverComparisonCounts{}) {
		t.Fatal(d.Completed, d.PreludeCompleted, d.Counts)
	}
	names := lpBatchSolveNames(output.String())
	want := make([]string, 0, 117)
	for _, i := range []int{6, 3, 9, 0, 5, 4, 2, 1, 8} {
		want = append(want, d.Files[i])
	}
	want = append(want, d.Files...)
	if !reflect.DeepEqual(names, want) {
		t.Fatal(names, want)
	}
	// --min does not override the normal loop's broken unprefixed minimum set.
	start := strings.Index(output.String(), "solving "+actual+"\n")
	tail := output.String()[start:]
	if !strings.Contains(tail, "Status: OPTIMAL\n") || !strings.Contains(tail, "cost = 14\n") || clock.calls != 2 {
		t.Fatal(tail, clock.calls)
	}
	if d.RunWithHome(home, true) != ErrExactVectorPending || d.Close() != nil {
		t.Fatal("single-use driver")
	}
}

func TestLPMPSBatchDefaultExactAndPreludeStops(t *testing.T) {
	home := t.TempDir()
	lpBatchPut(t, home, "l0redund.mps", lpProcessFixedGLPKModel(3))
	h := newLPPairHarness(t)
	var output bytes.Buffer
	clock := &lpMPSDriverClock{}
	d := NewMPSBatchDriver(lpMPSDriverArguments(t, true, false, true, true), h.m, MPSDriverConfig{Output: &output, Clock: clock.read})
	h.must(d.RunWithHome(home, true))
	if d.Completed != 108 || d.PreludeCompleted != 0 || clock.calls != 0 || !strings.Contains(output.String(), "status is OPTIMAL\n") || !strings.Contains(output.String(), "cost = 0\n") {
		t.Fatal(d.Completed, clock.calls, output.String())
	}
	// This source branch ignores minimum/setup/clock and uses default exact solve.
	h.must(d.Close())
	lpExactVectorNamedCensus(t, h, nil)

	home = t.TempDir()
	lpBatchPut(t, home, "l3.mps", lpLARFreeMPS(1))
	output.Reset()
	d = NewMPSBatchDriver(lpDriverArgs(t), nil, MPSDriverConfig{Output: &output})
	err := d.RunWithHome(home, true)
	wantErr := ErrDriverStatusWrong
	if sourceDebugEnabled {
		wantErr = ErrAssertion
	}
	if err != wantErr || d.PreludeCompleted != 1 || d.Completed != 0 || d.Current == nil || strings.Contains(output.String(), "exception:") {
		t.Fatal(err, d.PreludeCompleted, d.Completed, output.String())
	}
	if err = d.Close(); err != nil {
		t.Fatal(err)
	}
	// Missing HOME leaves the float prelude in the source invalid-index domain.
	output.Reset()
	d = NewMPSBatchDriver(lpDriverArgs(t), nil, MPSDriverConfig{Output: &output})
	if err = d.RunWithHome("", false); err != ErrBounds || d.PreludeCompleted != 0 {
		t.Fatal(err)
	}
	if err = d.Close(); err != nil {
		t.Fatal(err)
	}
	// Exact mode has an empty loop, so the same missing HOME returns normally.
	d = NewMPSBatchDriver(lpDriverArgs(t, "--mpq"), nil, MPSDriverConfig{Output: io.Discard})
	if err = d.RunWithHome("", false); err != nil || !d.closed || d.Completed != 0 {
		t.Fatal(err)
	}
}

func TestLPMPSBatchActualGLPKCatalogue(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	lpRequireGLPK(t)
	for _, exact := range []bool{false, true} {
		home := t.TempDir()
		files, _, err := FillDriverMPSCatalogue(home, true, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		for i, name := range files {
			if err = os.MkdirAll(filepath.Dir(name), 0700); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(name, []byte(lpProcessFixedGLPKModel(int64(3+i%7))), 0600); err != nil {
				t.Fatal(err)
			}
		}
		h := newLPPairHarness(t)
		args := lpMPSDriverArguments(t, exact, false, exact, true)
		args.usedFlags["--compare_with_glpk"] = true
		directory := filepath.Join(t.TempDir(), "new output")
		args.setValue("--out_dir", directory)
		var output bytes.Buffer
		clock := &lpMPSDriverClock{}
		d := NewMPSBatchDriver(args, h.m, MPSDriverConfig{Output: &output, Clock: clock.read})
		h.must(d.RunWithHome(home, true))
		if !d.closed || d.Process != nil || d.Current != nil || d.Completed != 108 || d.PreludeCompleted != 0 || d.Counts != (DriverComparisonCounts{108, 0, 0}) {
			t.Fatal(exact, d.Completed, d.Counts, output.String())
		}
		if !strings.HasPrefix(output.String(), "creating directory "+directory+"\nprocessing ") || !strings.HasSuffix(output.String(), "comparing with glpk: successes 108, failures 0, inconclusives 0\n") {
			t.Fatal(output.String())
		}
		for _, i := range []int{0, 10, 107} {
			name := filepath.Join(directory, DriverLPOutputName(exact, filepath.Base(files[i]), exact))
			value, err := ReadLPReportCost(name, io.Discard)
			h.must(err)
			want := float64(4*(3+i%7) + 2)
			if exact {
				want = 0
			}
			if value != want {
				t.Fatal(i, exact, value, want)
			}
		}
		h.must(d.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPMPSBatchNativeEnvironmentAndArgumentGuards(t *testing.T) {
	w := &lpResidualFailWriter{}
	d := NewMPSBatchDriver(lpDriverArgs(t, "--mpq"), nil, MPSDriverConfig{Output: lpResidualWriterOnly{w}})
	if err := d.Run(); err != lpErrResidualWrite {
		t.Fatal(err)
	}
	catalogueHome, present := os.LookupEnv("HOME")
	if present && (len(d.Files) != 108 || d.Files[0] != catalogueHome+"/projects/lean/src/tests/util/lp/l0redund.mps") {
		t.Fatal(d.Files)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d = NewMPSBatchDriver(nil, nil, MPSDriverConfig{Output: io.Discard})
	if d.Run() != ErrBounds || d.Close() != nil {
		t.Fatal("nil arguments")
	}
	d = NewMPSBatchDriver(lpDriverArgs(t, "--time_limit", "bad"), nil, MPSDriverConfig{Output: io.Discard})
	if err := d.RunWithHome("unused", true); err == nil || len(d.Files) != 0 {
		t.Fatal(err, d.Files)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLPMPSBatchDirectoryAndPathSources(t *testing.T) {
	for _, tc := range []struct{ name, parent, base string }{{"/x", "", "x"}, {"/dir/", "/dir", ""}, {"a//b", "a/", "b"}, {"C:/x/y", "C:/x", "y"}} {
		parent, base, err := SplitDriverMPSPath(tc.name, nil)
		if err != nil || parent != tc.parent || base != tc.base {
			t.Fatal(tc, parent, base, err)
		}
	}
	for _, name := range []string{"", "a", "C:\\x"} {
		report := "cannot find file name in " + name + "\n"
		for at := -1; at < len(report); at++ {
			var out bytes.Buffer
			var writer io.Writer = &out
			var fail *lpResidualFailWriter
			if at >= 0 {
				fail = &lpResidualFailWriter{remaining: at}
				writer = lpResidualWriterOnly{fail}
			}
			_, _, err := SplitDriverMPSPath(name, writer)
			if at < 0 {
				if err != ErrBounds || out.String() != report {
					t.Fatal(err, out.String())
				}
			} else if err != lpErrResidualWrite || fail.String() != report[:at] {
				t.Fatal(at, err, fail.String())
			}
		}
	}
	existing := t.TempDir()
	if err := PrepareDriverOutputDirectory(existing, nil); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(t.TempDir(), "regular file")
	if err := os.WriteFile(blocked, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	report := "creating directory " + blocked + "\nCannot open output directory \"" + blocked + "\"\n"
	for at := -1; at < len(report); at++ {
		var out bytes.Buffer
		var writer io.Writer = &out
		var fail *lpResidualFailWriter
		if at >= 0 {
			fail = &lpResidualFailWriter{remaining: at}
			writer = lpResidualWriterOnly{fail}
		}
		err := PrepareDriverOutputDirectory(blocked, writer)
		if at < 0 {
			if err != nil || out.String() != report {
				t.Fatal(err, out.String())
			}
		} else if err != lpErrResidualWrite || fail.String() != report[:at] {
			t.Fatal(at, err, fail.String())
		}
	}
	content, err := os.ReadFile(blocked)
	if err != nil || string(content) != "keep" {
		t.Fatal(err, string(content))
	}
}

func TestLPMPSBatchSummaryReportsAndLiveCounters(t *testing.T) {
	message := "cannot find home directory, don't know how to find the files"
	summary := "comparing with glpk: successes 0, failures 0, inconclusives 0\n"
	full := message + summary
	args := lpDriverArgs(t, "--compare_with_glpk", "--out_dir", t.TempDir())
	for at := 0; at < len(full); at++ {
		w := &lpResidualFailWriter{remaining: at}
		d := NewMPSBatchDriver(args, nil, MPSDriverConfig{Output: lpResidualWriterOnly{w}})
		if err := d.RunWithHome("", false); err != lpErrResidualWrite || w.String() != full[:at] || d.Completed != 0 {
			t.Fatal(at, err, w.String())
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
	w := &lpMPSDriverActionWriter{failFlush: 1}
	d := NewMPSBatchDriver(args, nil, MPSDriverConfig{Output: w})
	if err := d.RunWithHome("", false); err == nil || w.flushes != 1 {
		t.Fatal(err, w.flushes)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	w = &lpMPSDriverActionWriter{action: func(text string) error {
		switch text {
		case "comparing with glpk: successes ":
			d.Counts.Successes = 7
		case ", failures ":
			d.Counts.Failures = 9
		case ", inconclusives ":
			d.Counts.Inconclusives = 11
		}
		return nil
	}}
	d = NewMPSBatchDriver(args, nil, MPSDriverConfig{Output: w})
	if err := d.RunWithHome("", false); err != nil || !strings.HasSuffix(w.buffer.String(), "comparing with glpk: successes 7, failures 9, inconclusives 11\n") {
		t.Fatal(err, w.buffer.String())
	}
}

func TestLPMPSBatchChildFailurePrefixes(t *testing.T) {
	// The first rational default child reaches the source std::string bound
	// exception, which the surrounding const-char handler does not catch.
	home := t.TempDir()
	lpBatchPut(t, home, "l0redund.mps", strings.Replace(lpProcessFixedGLPKModel(3), "BOUNDS\n", "BOUNDS\n UP "+strings.Repeat(" ", 10)+"\n", 1))
	h := newLPPairHarness(t)
	d := NewMPSBatchDriver(lpDriverArgs(t, "--mpq"), h.m, MPSDriverConfig{Output: io.Discard})
	if err := d.RunWithHome(home, true); err != ErrMPSUnhandled || d.Current == nil || d.Completed != 0 {
		t.Fatal(err, d.Completed)
	}
	h.must(d.Close())
	lpExactVectorNamedCensus(t, h, nil)
	// Output directory creation failure is diagnosed but source still attempts
	// its first file and then propagates the integer report exception.
	var out bytes.Buffer
	blocked := filepath.Join(t.TempDir(), "output-file")
	if err := os.WriteFile(blocked, nil, 0600); err != nil {
		t.Fatal(err)
	}
	d = NewMPSBatchDriver(lpDriverArgs(t, "--compare_with_glpk", "--out_dir", blocked), nil, MPSDriverConfig{Output: &out})
	if err := d.RunWithHome(t.TempDir(), true); err != ErrDriverReport || d.Process == nil || d.Completed != 0 {
		t.Fatal(err, d.Completed, out.String())
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	// An actual writer refusal during a full-loop floating solve stops at samp2.
	home = t.TempDir()
	name := lpBatchPut(t, home, "samp2.mps", lpProcessFixedGLPKModel(3))
	w := &lpMPSDriverActionWriter{action: func(text string) error {
		if text == "solving "+name+"\n" {
			return lpErrResidualWrite
		}
		return nil
	}}
	d = NewMPSBatchDriver(lpDriverArgs(t), nil, MPSDriverConfig{Output: w})
	if err := d.RunWithHome(home, true); err != lpErrResidualWrite || d.Completed != 7 || d.PreludeCompleted != 9 {
		t.Fatal(err, d.Completed, d.PreludeCompleted)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLPMPSBatchPhysicalChildOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	home := t.TempDir()
	large := strings.Repeat("1234567890", 20) + "7/13"
	lpBatchPut(t, home, "l0redund.mps", strings.Replace(lpProcessFixedGLPKModel(3), lpExactMPSPair("x", "COST", "3"), lpExactMPSPair("x", "COST", large), 1))
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
				d := NewMPSBatchDriver(lpDriverArgs(t, "--mpq"), h.m, MPSDriverConfig{Output: io.Discard})
				h.budget.FailAfter(ordinal)
				err := d.RunWithHome(home, true)
				h.budget.FailAfter(-1)
				if err == nil {
					success = true
					if !d.closed || d.Completed != 108 || d.PreludeCompleted != 0 {
						t.Fatal("completion", d.Completed)
					}
				} else {
					failures++
					if !lpExactPriorityPhysicalRefusal(err) || d.Current == nil || d.RunWithHome(home, true) != ErrExactVectorPending {
						t.Fatal("physical child prefix", err)
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

func TestLPMPSBatchOlderCallerComparison(t *testing.T) {
	home := t.TempDir()
	lpBatchPut(t, home, "l0redund.mps", lpProcessFixedGLPKModel(3))
	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		h := newLPPairHarness(t)
		d := NewMPSBatchDriver(lpDriverArgs(t, "--mpq"), h.m, MPSDriverConfig{Output: io.Discard})
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
			_ = d.RunWithHome(home, true)
			if d.Current != nil && d.Current.calls.comparison != 0 {
				t.Fatal("child adopted caller comparison")
			}
		}
		h.must(d.Close())
		if older.Active && h.m.ComparisonInvocationState() != older {
			t.Fatal("batch cleanup consumed older caller")
		}
		h.must(caller.cleanupRaw(h.m))
		h.must(caller.cleanupRoots(h.m))
		lpExactVectorNamedCensus(t, h, nil)
	}
	if !matched {
		t.Fatal("no actual older caller comparison")
	}
}

func TestLPMPSBatchEveryPreludeFailurePrefix(t *testing.T) {
	for at, relative := range []string{"plan.mps", "l3.mps", "l0.mps", "l0redund.mps", "l4fix.mps", "l4.mps", "l2.mps", "l1.mps", "murtagh.mps"} {
		home := t.TempDir()
		text := lpLARFreeMPS(1)
		want := ErrDriverStatusWrong
		if sourceDebugEnabled {
			want = ErrAssertion
		}
		if at == 0 || at == 8 {
			text = strings.Replace(lpProcessFixedGLPKModel(3), "BOUNDS\n", "BOUNDS\n UP "+strings.Repeat(" ", 10)+"\n", 1)
			want = ErrMPSUnhandled
		}
		name := lpBatchPut(t, home, relative, text)
		var out bytes.Buffer
		d := NewMPSBatchDriver(lpMPSDriverArguments(t, false, false, false, false), nil, MPSDriverConfig{Output: &out})
		if err := d.RunWithHome(home, true); err != want || d.PreludeCompleted != at || d.Completed != 0 || d.Current == nil || d.Current.Name != name || strings.Contains(out.String(), "exception:") {
			t.Fatal(at, err, d.PreludeCompleted, out.String())
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
	// Creating-directory output fails before any actual mkdir or file child.
	directory := filepath.Join(t.TempDir(), "new-output")
	w := &lpResidualFailWriter{}
	d := NewMPSBatchDriver(lpDriverArgs(t, "--compare_with_glpk", "--out_dir", directory), nil, MPSDriverConfig{Output: lpResidualWriterOnly{w}})
	if err := d.RunWithHome(t.TempDir(), true); err != lpErrResidualWrite || d.Process != nil || d.Completed != 0 {
		t.Fatal(err, d.Completed)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}
