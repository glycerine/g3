package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// AUDIT.md FUZZ-F7 follow-up: (help-tactic) reproduces tactic_cmds.cpp
// help_tactic byte for byte: the combinator guide, every builtin tactic
// with its collect_param_descrs listing, the probes, and the closing quote
// after an untrimmed final newline. help_tactic_listing.expected is the
// stdout of the release build/z3 for help_tactic_listing.smt2.
func TestHelpTacticListingMatchesReference(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("test_vectors", "help_tactic_listing.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(filepath.Join("test_vectors", "help_tactic_listing.expected"))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = run(bytes.NewReader(script), &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), expected) {
		got, want := out.Bytes(), expected
		i := 0
		for i < len(got) && i < len(want) && got[i] == want[i] {
			i++
		}
		lo := i - 200
		if lo < 0 {
			lo = 0
		}
		t.Fatalf("help-tactic differs at byte %d\ngot:  %q\nwant: %q", i, got[lo:min(i+200, len(got))], want[lo:min(i+200, len(want))])
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		var native bytes.Buffer
		cmd := exec.Command(reference, "-in")
		cmd.Stdin = bytes.NewReader(script)
		cmd.Stdout = &native
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(native.Bytes(), expected) {
			t.Fatal("help_tactic_listing.expected is stale against the C++ reference")
		}
	}
}
