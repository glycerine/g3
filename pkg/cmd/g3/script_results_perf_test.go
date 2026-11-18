package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// perfScriptResults holds expected z3 outputs for vectors that drive
// performance work. They take tens of seconds even in z3, so they are kept
// out of scriptResults (run by every go test of this package) and are run
// only by TestScriptPerformanceBudgets, a long test.
var perfScriptResults = map[string]string{}

// perfScriptBudgets is the wall-clock budget of each perf vector in a
// release (untagged) build.
var perfScriptBudgets = map[string]time.Duration{}

func init() {
	// cmd_context::validate_model evaluates the 65-factor product at the
	// algebraic root x = 3^(1/65) with max_degree = UINT_MAX: 64 algebraic
	// multiplications, each factoring a degree-4225 resultant, almost all
	// of it in upolynomial mod_gcd (Zp Euclid, CRA, modular images).
	// build/z3 (release) on the development machine, 2026-11-18: 21.1 s
	// (rlimit-count 2156734). g3 at 2a387e415: 358 s; with the direct
	// coefficient kernels of math/polynomial and util/numeral: 31.4 s.
	perfScriptResults["model_validate_algebraic_power65.smt2"] = "sat\n"
	perfScriptBudgets["model_validate_algebraic_power65.smt2"] = 60 * time.Second
}

// TestScriptPerformanceBudgets runs each perf vector through g3, checks its
// output and fails if it exceeds its budget. With G3_CPP_REFERENCE set it
// also checks the expected output against the C++ executable.
func TestScriptPerformanceBudgets(t *testing.T) {
	if !testing.Short() {
		t.Skip("long performance test; runs only under -short")
	}
	if diagnostic.DebugBuild {
		t.Skip("budgets are for release builds; source debug builds are much slower")
	}
	reference := os.Getenv("G3_CPP_REFERENCE")
	for name, expected := range perfScriptResults {
		t.Run(name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("test_vectors", name))
			if err != nil {
				t.Fatal(err)
			}
			if reference != "" {
				cmd := exec.Command(reference, "-in")
				cmd.Stdin = bytes.NewReader(script)
				cppOutput, err := cmd.Output()
				if err != nil {
					t.Fatal("C++ reference", err, string(cppOutput))
				}
				if string(cppOutput) != expected {
					t.Fatalf("C++ reference output %q, want %q", cppOutput, expected)
				}
			}
			start := time.Now()
			var out bytes.Buffer
			if err := run(bytes.NewReader(script), &out); err != nil {
				t.Fatal(err, out.String())
			}
			elapsed := time.Since(start)
			if out.String() != expected {
				t.Fatalf("got %q, want %q", out.String(), expected)
			}
			budget := perfScriptBudgets[name]
			t.Logf("%s: %v (budget %v)", name, elapsed.Round(time.Millisecond), budget)
			if elapsed > budget {
				t.Fatalf("%s took %v, over its %v budget", name, elapsed, budget)
			}
		})
	}
}
