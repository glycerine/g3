package smt2

import (
	"strings"
	"testing"
)

func TestSMT2MarshalBooleanBenchmarkAndRoundTrip(t *testing.T) {
	_, manager, formula := marshalTestFormula(t, `(set-logic QF_UF)(declare-const p Bool)(assert (not p))`)
	text, ref, roundTrip := marshalTestRoundTrip(t, manager, formula)
	defer func() {
		if err := manager.DelRef(&ref); err != nil {
			t.Error(err)
		}
	}()
	for _, part := range []string{"(set-info :status unknown)", "(declare-fun p () Bool)", "(assert", "(not p)", "(check-sat)"} {
		if !strings.Contains(text, part) {
			t.Fatalf("marshal omitted %q:\n%s", part, text)
		}
	}
	if roundTrip != formula {
		t.Fatalf("canonical round trip changed handle: %v -> %v", formula, roundTrip)
	}
}

func TestSMT2MarshalTrueOmitsAssertionAndUnmarshalsToTrue(t *testing.T) {
	_, manager, truth := marshalTestFormula(t, `(assert true)`)
	text, ref, roundTrip := marshalTestRoundTrip(t, manager, truth)
	defer func() {
		if err := manager.DelRef(&ref); err != nil {
			t.Error(err)
		}
	}()
	if strings.Contains(text, "(assert") || !strings.Contains(text, "(check-sat)") {
		t.Fatalf("true benchmark %q", text)
	}
	if !manager.IsTrue(roundTrip) {
		t.Fatalf("empty assertion conjunction is not true: %v", roundTrip)
	}
}
