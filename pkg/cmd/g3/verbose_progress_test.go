//go:build g3trace

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The IF_VERBOSE progress lines ported for AUDIT.md FUZZ-F5 are compiled
// only into g3trace builds (owner decision 2026-11-15), so this test is
// tagged. It runs the g3 front end in a child process (timeit reports go to
// std::cerr, the verbose lines to verbose_stream, both standard error in
// the C++) and compares the whole standard error, with timeit's :time and
// memory fields removed, against what build/z3 printed for the same
// arguments: test_vectors/verbose_progress_<name>.stderr, and with
// G3_CPP_REFERENCE set also the live reference binary.

type verboseProgressCase struct {
	name, vector string
	args         []string
}

var verboseProgressCases = []verboseProgressCase{
	// (started "check-sat"), combined-solver, the three timeit lines,
	// (smt.configuring), (smt.collecting-features), (smt.searching),
	// (smt.final-check ...), the level-11 model dump.
	{"arith", "arith", []string{"-v:100"}},
	{"arith_v11", "arith", []string{"-v:11"}},
	// final-check per theory, (smt.mbqi) after the proto model dump, the
	// MBQI auxiliary context's own setup/timeit lines.
	{"quantified", "quantified", []string{"-v:100"}},
	{"quantified_no_mbqi", "quantified", []string{"-v:100", "smt.mbqi=false"}},
	// (smt.restarting ...), (smt.delete-inactive-clauses ...); at -v:100
	// the agility is fixed with two decimals because ~timeit left
	// std::fixed/setprecision(2) on std::cerr.
	{"pigeon8_v3", "pigeon8", []string{"-v:3"}},
	{"pigeon8_v100", "pigeon8", []string{"-v:100"}},
	// CFG_BASIC (setup_unknown) and :restart-outer for RS_IN_OUT_GEOMETRIC.
	{"pigeon8_basic", "pigeon8", []string{"-v:3", "auto_config=false"}},
	// (smt.working ...) every smt.tick conflicts of one bounded_search.
	{"pigeon9_v3", "pigeon9", []string{"-v:3"}},
	{"mixed_model", "mixed_model", []string{"-v:100"}},
	{"datatype", "datatype", []string{"-v:100"}},
	{"qf_s", "qf_s", []string{"-v:100"}},
	{"string", "string", []string{"-v:100"}},
	{"qf_bv", "qf_bv", []string{"-v:100"}},
	{"qf_lia", "qf_lia", []string{"-v:100"}},
	{"incremental", "incremental", []string{"-v:100"}},
	// CFG_LOGIC (push before check-sat): setup_default per logic, and
	// setup_seq_str's setup_str calling setup_arith again for z3str3.
	{"logic_incremental", "logic_incremental", []string{"-v:100"}},
	{"string_push", "string_push", []string{"-v:100"}},
	{"string_push_z3str3", "string_push", []string{"-v:100", "smt.string_solver=z3str3"}},
	{"array", "array", []string{"-v:100"}},
	{"usort", "usort", []string{"-v:100"}},
	{"unsat", "unsat", []string{"-v:100"}},
}

var verboseProgressMeasures = regexp.MustCompile(` :time [0-9.]+| :(before-|after-)?memory [0-9.]+`)

// normalizeVerboseProgress drops timeit's time and memory fields.
func normalizeVerboseProgress(log string) string {
	return verboseProgressMeasures.ReplaceAllString(log, "")
}

const verboseProgressChildEnv = "G3_TEST_VERBOSE_PROGRESS_ARGS"

// runVerboseProgressChild runs g3 with args in a child copy of the test
// binary and returns its standard error.
func runVerboseProgressChild(t *testing.T, args []string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestVerboseProgressMessages$")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), verboseProgressChildEnv+"="+strings.Join(args, "\x1f"))
	var out, diagnostics bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diagnostics
	if err := cmd.Run(); err != nil {
		t.Fatal(err, out.String(), diagnostics.String())
	}
	return diagnostics.String()
}

func TestVerboseProgressMessages(t *testing.T) {
	if args := os.Getenv(verboseProgressChildEnv); args != "" {
		os.Args = append([]string{"g3"}, strings.Split(args, "\x1f")...)
		main()
		return
	}
	reference := os.Getenv("G3_CPP_REFERENCE")
	for _, tc := range verboseProgressCases {
		t.Run(tc.name, func(t *testing.T) {
			path, err := filepath.Abs(filepath.Join("test_vectors", "verbose_progress_"+tc.vector+".smt2"))
			if err != nil {
				t.Fatal(err)
			}
			expected, err := os.ReadFile(filepath.Join("test_vectors", "verbose_progress_"+tc.name+".stderr"))
			if err != nil {
				t.Fatal(err)
			}
			args := append(append([]string(nil), tc.args...), path)
			got := normalizeVerboseProgress(runVerboseProgressChild(t, args))
			if got != string(expected) {
				t.Fatalf("g3 standard error\n%s\nwant (build/z3)\n%s", got, expected)
			}
			if reference == "" {
				return
			}
			var native, cppLog bytes.Buffer
			cmd := exec.Command(reference, args...)
			cmd.Stdout = &native
			cmd.Stderr = &cppLog
			if err := cmd.Run(); err != nil {
				t.Fatal(err, native.String(), cppLog.String())
			}
			if cpp := normalizeVerboseProgress(cppLog.String()); got != cpp {
				t.Fatalf("Go/C++ standard error differ\nGo:\n%s\nC++:\n%s", got, cpp)
			}
		})
	}
}
