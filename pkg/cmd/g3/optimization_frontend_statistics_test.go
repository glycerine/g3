package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// optimizationStatisticsLines keeps the deterministic part of opt_frontend's
// -st output: the result lines and every statistic except the process
// memory, allocation, time and rlimit counters.
func optimizationStatisticsLines(text string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		name, _, found := strings.Cut(line, ":")
		switch {
		case found && (name == "max memory" || name == "memory" || name == "num allocs" || name == "time" || name == "rlimit count"):
		case found && !strings.HasPrefix(line, " "):
			kept = append(kept, name+":"+strings.TrimLeft(line[len(name)+1:], " "))
		default:
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// gaps_misc: opt_frontend calls opt::context::optimize() whatever the
// objectives, so a WCNF or OPB input without objectives runs update_solver's
// incremental SAT solver too, and an input normalize() proves inconsistent
// checks the single hard constraint false. -st then shows those solvers'
// statistics (follow-up to AUDIT.md FUZZ_J-F6).
func TestOptimizationFrontendStatisticsWithoutObjectives(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"opb_no_objective_statistics.opb", "sat\nbinary propagations:1\ndecisions:1\nmk binary clause:2\nmk bool var:2\n"},
		{"opb_inconsistent_no_objective_statistics.opb", "unsat\nmk bool var:1\n"},
		{"opb_inconsistent_objective_statistics.opb", "unsat\nmk bool var:1\n  [(* (- 1) oo):oo]\n"},
		{"wcnf_all_hard_statistics.wcnf", "sat\nbinary propagations:2\ndecisions:4\ndel clause:1\nelim clauses:1\nelim literals:1\nmk binary clause:3\nmk bool var:8\nmk ternary clause:2\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("test_vectors", tc.name)
			var out bytes.Buffer
			if err := runArguments([]string{"-st", path}, strings.NewReader(""), &out); err != nil {
				t.Fatal(err, out.String())
			}
			if got := optimizationStatisticsLines(out.String()); got != tc.want {
				t.Fatalf("statistics\n%s\nwant\n%s\nfull output\n%s", got, tc.want, out.String())
			}
			reference := os.Getenv("G3_CPP_REFERENCE")
			if reference == "" {
				return
			}
			native, err := exec.Command(reference, "-st", path).Output()
			if err != nil {
				t.Fatal(err, string(native))
			}
			if got := optimizationStatisticsLines(string(native)); got != tc.want {
				t.Fatalf("C++ statistics\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}
