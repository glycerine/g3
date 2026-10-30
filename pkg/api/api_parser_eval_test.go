package api

import (
	"strings"
	"testing"
)

func TestAPIParserEvaluateUsesPersistentCommandSession(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	output, err := context.EvaluateSMT2("(set-logic QF_UF)(declare-const p Bool)(assert p)(check-sat)")
	astTestMust(t, err)
	if output != "sat\n" {
		t.Fatal("evaluation output", output)
	}
	output, err = context.EvaluateSMT2("(push)(assert (not p))(check-sat)(pop)(check-sat)")
	astTestMust(t, err)
	if output != "unsat\nsat\n" {
		t.Fatal("persistent evaluation output", output)
	}
	output, err = context.EvaluateSMT2("(assert")
	if err == nil || output != "" || !strings.Contains(err.Error(), "expected") {
		t.Fatal("parser diagnostic", output, err)
	}
}
