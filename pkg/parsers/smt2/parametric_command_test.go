package smt2

// Source: src/cmd_context/parametric_cmd.cpp and parametric_cmd.h.

import (
	"strings"
	"testing"
)

func TestParametricCommandsResetAndNormalizeOptions(t *testing.T) {
	script := `(simplify (+ 1 2) :print false)
(simplify (+ 3 4))
(declare-const p Bool)
(assert p)
(apply skip :print false)
(apply skip :print-dependencies true)`
	output, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err, output)
	}
	if !strings.HasPrefix(output, "7\n") || !strings.Contains(output, "(goals") {
		t.Fatalf("command-local options leaked or normalized option was ignored: %q", output)
	}
}

func TestParametricCommandRejectsInvalidKeywordAndValue(t *testing.T) {
	for _, script := range []string{
		`(apply skip :unknown true)`,
		`(apply skip :print 1)`,
		`(apply skip :timeout true)`,
	} {
		output, err := runApplyTacticScript(t, script)
		if err == nil {
			t.Fatalf("accepted invalid option in %s: %s", script, output)
		}
	}
}
