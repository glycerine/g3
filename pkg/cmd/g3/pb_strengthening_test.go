package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// FORK FIX (z3 4.7.1 bug; upstream removed this code in its pb rewrite, first
// in 4.8.5): justify() region-allocated the strengthening cut's
// justification, and the old reference aborted ("free(): invalid pointer")
// when the auxiliary lemma was deleted. The fixed fork heap-allocates a
// theory_lemma_justification and exits cleanly with the same proof.
func TestSMTLibRunnerPBStrengtheningProof(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("test_vectors", "pb_strengthening_proof.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	check := func(label, value string) {
		if !strings.HasPrefix(canonicalOutput(value), "unsat (cut_A cut_B) ") || !strings.Contains(value, "(proof") || !strings.Contains(value, "(asserted") || !strings.Contains(value, "false)") || strings.Count(value, "(") != strings.Count(value, ")") {
			t.Fatal(label, "missing complete PB cut proof/core", value)
		}
	}
	var native bytes.Buffer
	if err = run(bytes.NewReader(script), &native); err != nil {
		t.Fatal("Go PB proof failed", err, native.String())
	}
	check("Go", native.String())
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		// Avoid leaving a core file if an unfixed reference still aborts.
		cmd := exec.CommandContext(ctx, "sh", "-c", `ulimit -c 0; exec "$1" -in`, "g3-cpp-reference", reference)
		cmd.Stdin = bytes.NewReader(script)
		var out, diagnostics bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &diagnostics
		err = cmd.Run()
		check("C++", out.String())
		if err != nil || diagnostics.Len() != 0 {
			t.Fatal("unexpected C++ PB proof failure", err, diagnostics.String())
		}
		if out.String() != native.String() {
			t.Fatalf("C++ PB proof output differs\nGo: %s\nC++: %s", native.String(), out.String())
		}
	}
}
