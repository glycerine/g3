package smt2

import (
	"strings"
	"testing"
)

func TestSMT2DirectCheckSatPublishesUnsatAssumptions(t *testing.T) {
	script := "(set-option :produce-unsat-assumptions true)" +
		"(declare-const a Bool)(assert (not a))(check-sat a)(get-unsat-assumptions)"
	output, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output, "unsat\n(") || !strings.Contains(output, "a") {
		t.Fatal(output)
	}
}
