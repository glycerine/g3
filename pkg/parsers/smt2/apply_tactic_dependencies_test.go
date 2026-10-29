package smt2

import (
	"strings"
	"testing"
)

func TestSMT2ApplyTacticPrintsNamedDependencies(t *testing.T) {
	script := `(set-option :produce-unsat-cores true)(declare-const p Bool)(assert (! p :named named-p))(apply skip :print_dependencies true)`
	output, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err, output)
	}
	if !strings.Contains(output, "|- named-p\n  p") {
		t.Fatal(output)
	}
}
