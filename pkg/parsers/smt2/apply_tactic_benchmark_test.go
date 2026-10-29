package smt2

import (
	"strings"
	"testing"
)

func TestSMT2ApplyTacticPrintsReplayableBenchmark(t *testing.T) {
	script := `(set-logic QF_UF)(declare-const p Bool)(assert p)(apply skip :print false :print_benchmark true)`
	output, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err, output)
	}
	for _, fragment := range []string{"(set-logic QF_UF)", "(declare-fun p () Bool)", "(assert p)", "(check-sat)"} {
		if !strings.Contains(output, fragment) {
			t.Fatal("missing", fragment, output)
		}
	}
}

func TestSMT2ApplyTacticBenchmarkJoinsBranches(t *testing.T) {
	script := `(declare-const p Bool)(declare-const q Bool)(assert (or p q))(apply split-clause :print false :print_benchmark true)`
	output, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err, output)
	}
	if strings.Count(output, "(assert ") != 1 || !strings.Contains(output, "(or p q)") || !strings.HasSuffix(output, "(check-sat)\n") {
		t.Fatal(output)
	}
}
