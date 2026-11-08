package smt2

import (
	"strings"
	"testing"
)

func TestSMT2ApplyTacticPrintsResultGoals(t *testing.T) {
	output, err := runApplyTacticScript(t, `(declare-const p Bool)(assert (or p false))(apply simplify)`)
	if err != nil {
		t.Fatal(err, output)
	}
	if !strings.HasPrefix(output, "(goals\n") || !strings.Contains(output, "(goal\n  p\n  :precision precise :depth 1)") || !strings.HasSuffix(output, ")\n") {
		t.Fatal(output)
	}
}

func TestSMT2ApplyTacticPrintFalseSuppressesGoals(t *testing.T) {
	output, err := runApplyTacticScript(t, `(declare-const p Bool)(assert p)(apply skip :print false)`)
	if err != nil || output != "" {
		t.Fatal(err, output)
	}
}
