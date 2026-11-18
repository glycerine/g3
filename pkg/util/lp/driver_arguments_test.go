package lp

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func lpDriverArgs(t *testing.T, args ...string) *DriverArguments {
	t.Helper()
	a := NewDriverArguments(args)
	if !a.Parse() {
		t.Fatal(a.ErrorMessage, a.UsageString())
	}
	return a
}

func TestLPDriverArgumentCatalogue(t *testing.T) {
	flags := strings.Fields("-xyz_sample --test_swaps --test_perm --totalinf --smt --bp --min --max --mpq --test_lu --test_small_lu --test_larger_lu --test_larger_lu_with_holes --test_lp_0 --solve_some_mps --compare_with_glpk --dual --compare_with_primal --lar -tbq --randomize_lar --smap --term --eti --row_inf -pd --test_int_set --test_mpq --test_mpq_np --test_mpq_np_plus")
	values := strings.Fields("--density --harris_toler --checklu --partial_pivot --percent_for_enter --rep_frq --filelist --file --random_seed --max_iters --time_limit --simplex_strategy --test_file_directory --out_dir --maxng")
	if len(flags)+len(values) != 45 || len(lpDriverOptions) != 45 {
		t.Fatal("source catalogue extent")
	}
	for _, name := range flags {
		a := lpDriverArgs(t, name)
		if !a.OptionIsUsed(name) || a.OptionValue(name) != "" || len(a.freeArgs) != 0 {
			t.Fatal("flag", name)
		}
	}
	for _, name := range values {
		// An option-looking argument is still consumed as the preceding value.
		a := lpDriverArgs(t, name, "--min")
		if !a.OptionIsUsed(name) || a.OptionValue(name) != "--min" || a.OptionIsUsed("--min") {
			t.Fatal("valued option", name)
		}
	}
	if lpDriverArgs(t).OptionIsUsed("--absent") {
		t.Fatal("unknown option present")
	}
	usage := lpDriverArgs(t).UsageString()
	if !strings.HasPrefix(usage, "\nUsage:\n--test_mpq_np_plus /test rationals using plus instead of +=\n") || !strings.Contains(usage, "--density \"the percentage of non-zeroes in the matrix below which it is not dense\"\n") || strings.Count(usage, "\n") != 47 {
		t.Fatal(usage)
	}
}

func TestLPDriverArgumentSourcePrefixesAndAccumulation(t *testing.T) {
	input := []string{"lp-test", "--min", "--density", "25", "--density", "70", "file.mps", "--rep_frq", ""}
	a := NewDriverArguments(input)
	input[0] = "mutated argv"
	if !a.Parse() || a.OptionValue("--density") != "70" || !a.OptionIsUsed("--rep_frq") || a.OptionValue("--rep_frq") != "" || !reflect.DeepEqual(a.freeArgs, []string{"lp-test", "file.mps"}) {
		t.Fatal("source argument acquisition or replacement")
	}
	if !a.Parse() || !reflect.DeepEqual(a.freeArgs, []string{"lp-test", "file.mps", "lp-test", "file.mps"}) {
		t.Fatal("repeated parse cleared or replaced accumulated state")
	}
	a = NewDriverArguments([]string{"--min", "--density", "12", "--file"})
	if a.Parse() || a.ErrorMessage != "Argument is missing after --file" || !a.OptionIsUsed("--min") || a.OptionValue("--density") != "12" || a.OptionIsUsed("--file") {
		t.Fatal("missing value prefix")
	}
	a = NewDriverArguments([]string{"--bogus", "//bad", "\\visible", "ordinary", "", "--min"})
	if a.Parse() || a.ErrorMessage != "" || !a.OptionIsUsed("--min") || !strings.HasPrefix(a.UsageString(), "Unknown options:--bogus,\\visible,\nUsage:\n") || strings.Contains(strings.Split(a.UsageString(), "Usage:")[0], "//bad") {
		t.Fatal("parse/usage prefix mismatch")
	}
	if !lpDriverArgs(t, "\\native", "/absolute/file").Parse() {
		t.Fatal("source accepted path rejected")
	}
}

type lpDriverFlushWriter struct {
	bytes.Buffer
	flushes, failAt int
}

func (w *lpDriverFlushWriter) Flush() error {
	w.flushes++
	if w.flushes == w.failAt {
		return lpErrResidualWrite
	}
	return nil
}

func TestLPDriverArgumentPrintAndOutputFailures(t *testing.T) {
	a := lpDriverArgs(t, "program", "--mpq", "--min", "--density", "25", "--file", "input.mps", "free")
	// libstdc++ node order, as build/test-z3 prints it (AUDIT.md util_lp_01-F1).
	want := "options are: \n--min\n--mpq\n--file input.mps\n--density 25\nfree arguments are: \nprogram \nfree \n"
	var out bytes.Buffer
	if err := a.Print(&out); err != nil || out.String() != want {
		t.Fatal(err, out.String())
	}
	for accepted := 0; accepted <= len(want); accepted++ {
		w := &lpResidualFailWriter{remaining: accepted}
		err := a.Print(lpResidualWriterOnly{writer: w})
		if accepted < len(want) && err != lpErrResidualWrite || accepted == len(want) && err != nil || w.String() != want[:accepted] {
			t.Fatal(accepted, err, w.String())
		}
	}
	for line := 1; line <= 8; line++ {
		w := &lpDriverFlushWriter{failAt: line}
		if err := a.Print(w); err != lpErrResidualWrite || w.flushes != line || strings.Count(w.String(), "\n") != line {
			t.Fatal("endl flush failure", line, err)
		}
	}
	out.Reset()
	if err := lpDriverArgs(t).Print(&out); err != nil || out.String() != "no options are given\n" {
		t.Fatal(err, out.String())
	}
	if err := lpDriverArgs(t).Print(nil); err != nil {
		t.Fatal(err)
	}
}

func TestLPDriverArgumentNumericDomains(t *testing.T) {
	for _, tc := range []struct {
		text string
		want uint32
	}{{"0", 0}, {"+17", 17}, {" -1 ", ^uint32(0)}, {"-2147483648", 2147483648}, {"2147483647", 2147483647}} {
		a := lpDriverArgs(t, "--max_iters", tc.text, "--time_limit", "19")
		time, maximum, err := a.TimeAndIterationLimits()
		if err != nil || time != 19 || maximum != tc.want {
			t.Fatal(tc, time, maximum, err)
		}
	}
	for _, bad := range []string{"junk", "12junk", "2147483648", "-2147483649", " "} {
		a := lpDriverArgs(t, "--max_iters", bad, "--time_limit", "19")
		if time, _, err := a.TimeAndIterationLimits(); err == nil || time != 0 {
			t.Fatal("max parsed before time", bad, time, err)
		}
		a = lpDriverArgs(t, "--max_iters", "23", "--time_limit", bad)
		if _, maximum, err := a.TimeAndIterationLimits(); err == nil || maximum != 23 {
			t.Fatal("completed max survives time error", bad, maximum, err)
		}
	}
	for _, a := range []*DriverArguments{lpDriverArgs(t), lpDriverArgs(t, "--max_iters", "", "--time_limit", "")} {
		if time, maximum, err := a.TimeAndIterationLimits(); err != nil || time != 0 || maximum != 0 {
			t.Fatal(time, maximum, err)
		}
	}
	if value, present, err := lpDriverArgs(t, "--harris_toler", " +1.25e-3 ").doubleValue("--harris_toler"); err != nil || !present || value != 0.00125 {
		t.Fatal(value, present, err)
	}
	if _, present, err := lpDriverArgs(t, "--harris_toler", "").doubleValue("--harris_toler"); present || err != nil {
		t.Fatal(present, err)
	}
	if _, _, err := lpDriverArgs(t, "--harris_toler", "1.2suffix").doubleValue("--harris_toler"); err == nil {
		t.Fatal("malformed libc numeric prefix was emulated")
	}
}

// AUDIT.md util_lp_01-F1: argument_parser::print walks the valued options in
// libstdc++ unordered_map order, as build/test-z3 prints them.
func TestLPDriverArgumentPrintValuedOrder(t *testing.T) {
	a := NewDriverArguments([]string{"--term", "--min", "--dual", "--file", "x", "--max_iters", "3"})
	if !a.Parse() {
		t.Fatal(a.ErrorMessage)
	}
	var out bytes.Buffer
	if err := a.Print(&out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "options are: \n--dual\n--min\n--term\n--max_iters 3\n--file x\n" {
		t.Fatalf("%q", out.String())
	}
	usage := NewDriverArguments([]string{"--bogus"})
	usage.Parse()
	if !strings.HasPrefix(usage.UsageString(), "Unknown options:--bogus,\nUsage:\n--test_mpq_np_plus /test rationals using plus instead of +=\n--test_swaps /") {
		t.Fatalf("%q", usage.UsageString())
	}
}
