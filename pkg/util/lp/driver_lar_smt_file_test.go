package lp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLPLARSMTFileSourceSecondMPSRead(t *testing.T) {
	for world := 0; world < 40; world++ {
		h := newLPPairHarness(t)
		input := fmt.Sprintf(":formula (and (<= (+ (* %d x) (* (~ %d) y) %d) 999))", world+1, world+2, world+10)
		name := lpDriverListFile(t, input)
		var out bytes.Buffer
		clock := &lpMPSDriverClock{failAt: 1}
		d := NewLARFileDriver(name, lpDriverArgs(t, "lp-test", "--smt", "--randomize_lar"), h.m, MPSDriverConfig{Output: &out, Clock: clock.read})
		err := d.Run()
		if sourceDebugEnabled {
			if err != ErrAssertion || d.Solver == nil || d.smtCreator == nil || d.reader != nil || d.closed {
				t.Fatal(world, err)
			}
		} else {
			h.must(err)
			if out.String() != lpLARSMTFileSecondMPSReadText(world) || !d.closed || d.Status != StatusUnknown || d.Runner != nil {
				t.Fatal(world, out.String(), d.Status)
			}
		}
		if clock.calls != 0 || strings.Contains(out.String(), "status is ") {
			t.Fatal("source must not solve this SMT file", clock.calls, out.String())
		}
		h.must(d.Close())
		if d.Run() != ErrExactVectorPending {
			t.Fatal("driver retried")
		}
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPLARSMTPolyglotRunAndComparison(t *testing.T) {
	for _, compare := range []bool{false, true} {
		h := newLPPairHarness(t)
		// Both source readers accept this file: SMT sees an empty conjunction,
		// MPS skips that prefix and finds its own NAME/ROWS/COLUMNS sections.
		name := lpDriverListFile(t, ":formula (and)\n"+lpLARFreeMPS(2))
		flags := []string{"lp-test", "--smt", "--randomize_lar"}
		if compare {
			flags = append(flags, "--compare_with_primal")
		}
		var out bytes.Buffer
		clock := &lpMPSDriverClock{}
		var d *LARFileDriver
		read := func() (uint64, error) {
			if d.smtCreator != nil || d.creator != nil || d.reader == nil || d.Solver == nil {
				t.Fatal("creator scope not closed before second read/run")
			}
			return clock.read()
		}
		d = NewLARFileDriver(name, lpDriverArgs(t, flags...), h.m, MPSDriverConfig{Output: &out, Clock: read})
		h.must(d.Run())
		want := "status is OPTIMAL, processed for 2 seconds, and 1 iterations\nchecking randomize\n"
		reads := 3
		if compare {
			want = "status = OPTIMAL\n"
			reads = 0
		}
		if out.String() != want || clock.calls != reads || d.Status != StatusOptimal || !d.closed {
			t.Fatal(compare, out.String(), clock.calls, d.Status)
		}
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPLARSMTFileInputDispatch(t *testing.T) {
	first := lpDriverListFile(t, ":formula (and)")
	second := lpDriverListFile(t, ":formula (and)")
	list := lpDriverListFile(t, first+"\n"+second+"\n"+first+"\n\nmissing")
	for _, single := range []bool{false, true} {
		h := newLPPairHarness(t)
		flags := []string{"lp-test", "--smt", "--filelist", list}
		want := []string{first, second, first}
		if single {
			flags = append(flags, "--file", second)
			want = []string{second}
		}
		var out bytes.Buffer
		d := NewLARInputDriver(lpDriverArgs(t, flags...), h.m, MPSDriverConfig{Output: &out})
		h.must(d.Run())
		if !reflect.DeepEqual(d.Files, want) || d.Completed != len(want) || !d.closed || d.Current != nil || strings.Count(out.String(), "cannot read from file") != len(want) {
			t.Fatal(d.Files, d.Completed, out.String())
		}
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPLARSMTFileBadCreationAndCleanup(t *testing.T) {
	for _, profile := range []string{"missing", "invalid-top", "valid-mps-only", "read-close"} {
		h := newLPPairHarness(t)
		input := ":formula (or)"
		if profile == "valid-mps-only" {
			input = lpLARFreeMPS(2)
		} else if profile == "read-close" {
			input = ":formula\n"
		}
		name := lpDriverListFile(t, input)
		if profile == "missing" {
			name += ".missing"
		}
		w := &lpMPSDriverActionWriter{}
		var d *LARFileDriver
		w.action = func(text string) error {
			if profile == "read-close" && text == "empty formula\n" {
				return d.smtCreator.file.Close()
			}
			return nil
		}
		d = NewLARFileDriver(name, lpDriverArgs(t, "lp-test", "--smt"), h.m, MPSDriverConfig{Output: w})
		err := d.Run()
		switch profile {
		case "missing":
			h.must(err)
			want := "cannot open file " + name + "\nsetting error\ncannot process " + name + "\nsetting m_is_OK to false\n"
			if w.buffer.String() != want || !d.closed {
				t.Fatal(w.buffer.String())
			}
		case "invalid-top":
			h.must(err)
			if !strings.Contains(w.buffer.String(), "cannot process "+name+"\n") || !d.closed {
				t.Fatal(w.buffer.String())
			}
		case "valid-mps-only":
			if err != ErrBounds || d.Solver != nil || d.Runner == nil || d.reader == nil || d.smtCreator != nil {
				t.Fatal(err, w.buffer.String())
			}
		case "read-close":
			if !errors.Is(err, os.ErrClosed) || d.smtCreator == nil || d.reader != nil {
				t.Fatal(err)
			}
			if err := d.Close(); !errors.Is(err, os.ErrClosed) || d.smtCreator == nil || d.closed {
				t.Fatal(err)
			}
		}
		h.must(d.Close())
		h.must(d.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPLARSMTFileReportFailures(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, profile := range []string{"empty-smt", "polyglot", "missing"} {
		input := ":formula (and)"
		if profile == "polyglot" {
			input += "\n" + lpLARFreeMPS(2)
		}
		name := lpDriverListFile(t, input)
		if profile == "missing" {
			name += ".missing"
		}
		// The failed getline after the unterminated line keeps it, as
		// build/test-z3 shows (AUDIT.md util_lp_04-F3).
		want := "setting m_is_OK to false\ncannot read from file\nline is too short for a column\nformula (and)\nline number is 3\nsetting m_is_OK to false\nline is too short\nformula (and)\nline number is 3\nsetting m_is_OK to false\n"
		flushes := 0
		if profile == "polyglot" {
			want = "status is OPTIMAL, processed for 2 seconds, and 1 iterations\nchecking randomize\n"
			flushes = 2
		} else if profile == "missing" {
			want = "cannot open file " + name + "\nsetting error\ncannot process " + name + "\nsetting m_is_OK to false\n"
			flushes = 3
		}
		for accepted := 0; accepted <= len(want); accepted++ {
			h := newLPPairHarness(t)
			w := &lpResidualFailWriter{remaining: accepted}
			d := NewLARFileDriver(name, lpDriverArgs(t, "lp-test", "--smt", "--randomize_lar"), h.m, MPSDriverConfig{Output: lpResidualWriterOnly{writer: w}, Clock: (&lpMPSDriverClock{}).read})
			err := d.Run()
			if w.String() != want[:accepted] || accepted < len(want) && err != lpErrResidualWrite || accepted == len(want) && err != nil {
				t.Fatal(profile, accepted, err, w.String())
			}
			h.must(d.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
		for flush := 1; flush <= flushes; flush++ {
			h := newLPPairHarness(t)
			w := &lpMPSDriverActionWriter{failFlush: flush}
			d := NewLARFileDriver(name, lpDriverArgs(t, "lp-test", "--smt", "--randomize_lar"), h.m, MPSDriverConfig{Output: w, Clock: (&lpMPSDriverClock{}).read})
			if err := d.Run(); err != lpErrResidualWrite || w.flushes != flush {
				t.Fatal(profile, flush, err)
			}
			h.must(d.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLPLARSMTFilePhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	// This MPS value is reparsed for every fault ordinal; twenty blocks retain
	// the multi-limb allocation path exercised by the file driver.
	large := strings.Repeat("1234567890", 20) + "7/13"
	mps := strings.Replace(lpLARFreeMPS(2), lpExactMPSPair("x0", "COST", "1"), lpExactMPSPair("x0", "COST", large), 1)
	for _, phase := range []string{"constraints", "empty-comparison"} {
		formula := ":formula (and)\n"
		if phase == "constraints" {
			formula = ":formula (and (<= (+ " + strings.Repeat("(* (~ -2147483648) x) ", 24) + "2147483647 2147483647) 999))\n"
		}
		name := lpDriverListFile(t, formula+mps)
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
					flags := []string{"lp-test", "--smt", "--randomize_lar"}
					if phase == "empty-comparison" {
						flags = append(flags, "--compare_with_primal")
					}
					d := NewLARFileDriver(name, lpDriverArgs(t, flags...), h.m, MPSDriverConfig{Output: io.Discard, Clock: (&lpMPSDriverClock{}).read})
					h.budget.FailAfter(ordinal)
					err := d.Run()
					h.budget.FailAfter(-1)
					if phase == "constraints" && sourceDebugEnabled && err == ErrAssertion {
						success = true
						if d.Solver == nil || d.smtCreator == nil || d.reader != nil {
							t.Fatal("checked factory boundary")
						}
					} else if err == nil {
						success = true
						if d.Status != StatusOptimal || !d.closed {
							t.Fatal(d.Status, d.closed)
						}
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
					}
					h.must(d.Close())
					lpExactVectorNamedCensus(t, h, nil)
				}) {
					t.FailNow()
				}
			}
			if !success || direct && failures == 0 {
				t.Fatal(phase, direct, success, failures)
			}
		}
	}
}

func TestLPLARSMTFileOlderCallerComparison(t *testing.T) {
	name := lpDriverListFile(t, ":formula (and)\n"+lpLARFreeMPS(2))
	for _, compare := range []bool{false, true} {
		matched := false
		for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
			h := newLPPairHarness(t)
			flags := []string{"lp-test", "--smt"}
			if compare {
				flags = append(flags, "--compare_with_primal")
			}
			d := NewLARFileDriver(name, lpDriverArgs(t, flags...), h.m, MPSDriverConfig{Output: io.Discard, Clock: (&lpMPSDriverClock{}).read})
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
				if d.Solver != nil && d.Solver.calls.comparison != 0 || d.reader != nil && d.reader.calls.comparison != 0 {
					t.Fatal("SMT file child adopted receipt")
				}
			}
			h.must(d.Close())
			if older.Active && h.m.ComparisonInvocationState() != older {
				t.Fatal("SMT cleanup consumed older receipt")
			}
			h.must(caller.cleanupRaw(h.m))
			h.must(caller.cleanupRoots(h.m))
			lpExactVectorNamedCensus(t, h, nil)
		}
		if !matched {
			t.Fatal("no actual older comparison", compare)
		}
	}
}

func TestLPLARSMTInputFailedSecondCreatorClose(t *testing.T) {
	h := newLPPairHarness(t)
	first := lpDriverListFile(t, ":formula (and)")
	second := lpDriverListFile(t, ":formula\n")
	list := lpDriverListFile(t, first+"\n"+second+"\n"+first)
	var d *LARInputDriver
	w := &lpMPSDriverActionWriter{}
	w.action = func(text string) error {
		if text == "empty formula\n" {
			return d.Current.smtCreator.file.Close()
		}
		return nil
	}
	d = NewLARInputDriver(lpDriverArgs(t, "lp-test", "--smt", "--filelist", list), h.m, MPSDriverConfig{Output: w})
	if err := d.Run(); !errors.Is(err, os.ErrClosed) || d.Completed != 1 || d.Current == nil || d.Current.smtCreator == nil {
		t.Fatal(err, d.Completed)
	}
	child := d.Current
	if err := d.Close(); !errors.Is(err, os.ErrClosed) || d.Current != child || d.closed || child.closed {
		t.Fatal(err)
	}
	h.must(d.Close())
	if d.Current != nil || !d.closed {
		t.Fatal("partial child retained after retry")
	}
	lpExactVectorNamedCensus(t, h, nil)
}

// lpLARSMTFileSecondMPSReadText is build/test-z3's text for the MPS read of
// the unterminated one-line SMT file: the stale last line is re-read after
// the failed getline (AUDIT.md util_lp_04-F3).
func lpLARSMTFileSecondMPSReadText(world int) string {
	line := fmt.Sprintf("line = formula (and (<= (+ (* %d x) (* (~ %d) y) %d) 999)), line number is 3\n", world+1, world+2, world+10)
	block := "cannot find <= (+ (*\nsetting m_is_OK to false\ncannot understand this line\n" + line
	return "setting m_is_OK to false\ncannot read from file\n" + block + block
}
