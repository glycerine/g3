package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// datalogStatisticsLines returns the "dl." statistics lines of a
// fixedpoint.print_statistics=true run, without column padding (the time
// and memory lines are not deterministic).
func datalogStatisticsLines(t *testing.T, vector string, options ...string) string {
	t.Helper()
	var out bytes.Buffer
	args := append([]string{"fixedpoint.print_statistics=true"}, options...)
	args = append(args, filepath.Join("test_vectors", vector))
	if err := runArguments(args, strings.NewReader(""), &out); err != nil {
		t.Fatal(err, out.String())
	}
	var lines []string
	for _, line := range strings.Split(out.String(), "\n") {
		fields := strings.Fields(strings.Trim(line, " ()"))
		if len(fields) == 2 && strings.HasPrefix(fields[0], ":dl.") {
			lines = append(lines, fields[0]+" "+fields[1])
		}
	}
	return strings.Join(lines, "\n")
}

// AUDIT.md muz_rel_03-F1: instr_io load of an empty relation leaves the
// register null, so filter_by_negation does not count (z3 prints no
// dl.filter_by_negation entry).
func TestRelLoadEmptyStatistics(t *testing.T) {
	want := ":dl.joins 3\n:dl.project_rename 1\n:dl.unary_singleton 4\n:dl.union 4"
	for _, options := range [][]string{nil, {"fixedpoint.datalog.default_table=hashtable"}} {
		if got := datalogStatisticsLines(t, "rel_load_empty_stats.smt2", options...); got != want {
			t.Fatalf("%v: dl statistics\n%s\nwant\n%s", options, got, want)
		}
	}
}
