package smt2

import (
	"strings"
	"testing"
)

func TestSMT2ApplyTacticPrintsStatisticsAfterGoals(t *testing.T) {
	output, err := runApplyTacticScript(t, `(declare-const p Bool)(assert p)(apply simplify :print_statistics true)`)
	if err != nil {
		t.Fatal(err, output)
	}
	if !strings.Contains(output, ")\n(") || !strings.Contains(output, ":rlimit-count") {
		t.Fatal(output)
	}
}
