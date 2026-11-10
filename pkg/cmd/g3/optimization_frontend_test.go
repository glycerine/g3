package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOptimizationFrontendArguments(t *testing.T) {
	for _, tc := range []struct{ name, option, input, value string }{
		{"opb", "-bpo", "min: 1 x1 +2 x2;\n1 x1 +1 x2 >= 1;\n", "sat\n   1\n"},
		{"opb-negative", "-bpo", "min: -3 x1;\n1 x1 >= 1;\n", "sat\n   (- 3)\n"},
		{"opb-final-objective-without-semicolon", "-bpo", "min: 1 x1", "sat\n   0\n"},
		{"opb-constraints-without-semicolons", "-bpo", "1 x1 >= 1\n1 ~x1 >= 1", "unsat\n"},
		{"wcnf", "-wcnf", "p wcnf 1 3 10\n10 1 0\n2 -1 0\n3 1 0\n", "sat\n   2\n"},
		{"wcnf-unsat-zero-weight", "-wcnf", "p wcnf 1 3 10\n10 1 0\n10 -1 0\n0 1 0\n", "unsat\n   0\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := runArguments([]string{tc.option, "-in"}, strings.NewReader(tc.input), &output); err != nil {
				t.Fatal(err, output.String())
			}
			if text := output.String(); text != tc.value {
				t.Fatalf("optimization output: %q", text)
			}
		})
	}
}

func TestOptimizationFrontendFileTypeDetection(t *testing.T) {
	for _, tc := range []struct{ filename, input string }{
		{"problem.opb", "1 x1 >= 1;\n"},
		{"problem.wcnf", "p wcnf 1 1 5\n5 1 0\n"},
	} {
		t.Run(tc.filename, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.filename)
			if err := os.WriteFile(path, []byte(tc.input), 0600); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := runArguments([]string{path}, strings.NewReader(""), &output); err != nil {
				t.Fatal(err)
			}
			if output.String() != "sat\n" {
				t.Fatalf("unexpected output: %q", output.String())
			}
		})
	}
}

func TestOptimizationFrontendWCNFSortMax(t *testing.T) {
	input := "p wcnf 2 3 10\n10 -1 -2 0\n5 1 0\n3 2 0\n"
	var output bytes.Buffer
	if err := runArguments([]string{"opt.maxsat_engine=sortmax", "-wcnf", "-in"}, strings.NewReader(input), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if output.String() != "sat\n   3\n" {
		t.Fatalf("sortmax output: %q", output.String())
	}
}

func TestOptimizationFrontendResourceLimitedOPBBounds(t *testing.T) {
	input := "min: 1 ~x1 +2 ~x2 +3 ~x3 +4 ~x4 +5 ~x5 +6 ~x6;\n" +
		"1 x1 +1 x2 +1 x3 +1 x4 +1 x5 +1 x6 <= 5;\n"
	// Work limits select a search checkpoint, not an invariant model order
	// (z3 4.7.1 finishes both runs within these limits). The seeded run
	// keeps coverage of an interrupted intermediate bound (cost 4 since
	// Boolean class propagation follows propagate_bool_var_enode), and the
	// default path the tighter cost-1 bound. Neither may report an optimum.
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		// optimize() normalizes (simplify tactic) and checks the hard
		// constraints before MaxRes, which spends part of each budget. The
		// normalized problem is propositional, so update_solver moves it to
		// inc_sat_solver (the smt.* phase options no longer steer it).
		{"default-phase", []string{"rlimit=415", "-bpo", "-in"}, "unknown\n  [0:1]\n"},
		{"seeded-phase", []string{"rlimit=340", "smt.phase_selection=5", "smt.random_seed=0", "-bpo", "-in"}, "unknown\n  [0:6]\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := runArguments(test.args, strings.NewReader(input), &output); err != nil {
				t.Fatal(err, output.String())
			}
			if got := output.String(); got != test.want {
				t.Fatalf("resource-limited OPB output = %q, want %q", got, test.want)
			}
		})
	}
}

func TestOptimizationFrontendResourceLimitDuringMaxResCore(t *testing.T) {
	input := "min: 1 ~x1 +2 ~x2 +3 ~x3 +4 ~x4 +5 ~x5 +6 ~x6;\n" +
		"1 x1 +1 x2 +1 x3 +1 x4 +1 x5 +1 x6 <= 5;\n"
	var output bytes.Buffer
	if err := runArguments([]string{"rlimit=415", "-bpo", "-in"}, strings.NewReader(input), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if got, want := output.String(), "unknown\n  [0:1]\n"; got != want {
		t.Fatalf("resource-limited OPB output = %q, want %q", got, want)
	}
}

func TestOptimizationFrontendResourceLimitedSortMaxBounds(t *testing.T) {
	input := "p wcnf 6 7 100\n100 -1 -2 -3 -4 -5 -6 0\n" +
		"1 1 0\n1 2 0\n1 3 0\n1 4 0\n1 5 0\n1 6 0\n"
	var output bytes.Buffer
	args := []string{"rlimit=2200", "opt.maxsat_engine=sortmax", "-wcnf", "-in"}
	if err := runArguments(args, strings.NewReader(input), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if got, want := output.String(), "unknown\n  [0:3]\n"; got != want {
		t.Fatalf("resource-limited SortMax output = %q, want %q", got, want)
	}
}

// maxres found_optimum reports the cost of the best model, not the core
// lower bound, which can overshoot it (seed 5 drove it to 8 while the model
// x=9 costs 6, printing objective 7).
func TestOptimizationBitVectorMaxResReportsBestModelCost(t *testing.T) {
	input := "(set-option :produce-models true)\n(set-logic QF_BV)\n(declare-const x (_ BitVec 4))\n" +
		"(assert (bvule x #x9))\n(maximize x)\n(check-sat)\n(get-objectives)\n(get-value (x))\n"
	for _, seed := range []string{"0", "5"} {
		var output bytes.Buffer
		if err := runArguments([]string{"smt.random_seed=" + seed, "-in"}, strings.NewReader(input), &output); err != nil {
			t.Fatal(err, output.String())
		}
		if got, want := output.String(), "sat\n(objectives\n (x 9)\n)\n((x #x9))\n"; got != want {
			t.Fatalf("seed %s: output = %q, want %q", seed, got, want)
		}
	}
}
