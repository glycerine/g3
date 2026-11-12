package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSMTLibRunnerQSATAndQuantifierElimination(t *testing.T) {
	for _, tc := range []struct {
		name, want, referenceWant string
		referenceQSAT             bool
	}{
		{"qsat_alternating_unsat.smt2", "unsat\n", "", false},
		{"qsat_integer_unsat.smt2", "unsat\n", "", false},
		{"qsat_model.smt2", "sat\n((k 3))\n", "", false},
		{"qe2_linear_projection.smt2", "sat\n", "", false},
		{"qe2_nested_unsat.smt2", "unsat\n", "", false},
		{"qe_rec_blocked_terms.smt2", "unsat\n", "", false},
		{"qe_rec_free_boolean.smt2", "sat\n", "", true},
		{"qsat_scopes.smt2", "sat\nunsat\nsat\n((k 3))\n", "", false},
		// nlqsat's is_pure_proc rejects the Int numerals' to_real coercions
		// ("not NRA"), so those goals end unknown, as in the C++ build.
		{"nlqsat_nonlinear_quantifiers.smt2", "unknown\nunknown\nsat\nunsat\nunknown\nunknown\n", "", false},
		{"nra_portfolio.smt2", "sat\nsat\nunsat\nsat\n", "", false},
		{"nra_solver_quantifiers.smt2", "sat\nsat\nunsat\nsat\n", "", false},
		{"nlqsat_models.smt2", "sat\n((k 3.0))\nsat\n((k 3.0))\n", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("test_vectors", tc.name))
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err = run(bytes.NewReader(script), &out); err != nil {
				t.Fatal(err, out.String())
			}
			if out.String() != tc.want {
				t.Fatalf("native output %q; want %q", out.String(), tc.want)
			}
			if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
				input := script
				// The old qe_rec entry segfaults on its uninitialized free-Boolean
				// level. Its qsat tactic independently decides the same input.
				if tc.referenceQSAT {
					input = bytes.ReplaceAll(input, []byte("(then qe_rec smt)"), []byte("qsat"))
				}
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, reference, "-in")
				cmd.Stdin = bytes.NewReader(input)
				cpp, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatal(err, string(cpp))
				}
				referenceWant := tc.referenceWant
				if referenceWant == "" {
					referenceWant = out.String()
				}
				if string(cpp) != referenceWant {
					t.Fatalf("unexpected C++ output\nGo: %s\nC++: %s\nwant C++: %s", out.String(), cpp, referenceWant)
				}
			}
		})
	}
}

func TestSMTLibRunnerNLQSATProjection(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("test_vectors", "nlqsat_projection.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = run(bytes.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	// Every goal holds a to_real coercion of an Int numeral, which nlqsat's
	// is_pure_proc rejects as "not NRA" (nlqsat.cpp:482-507).
	if want := "unknown\nunknown\nunknown\n"; out.String() != want {
		t.Fatalf("native output %q; want %q", out.String(), want)
	}
}
