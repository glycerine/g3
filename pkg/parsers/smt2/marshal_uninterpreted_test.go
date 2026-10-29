package smt2

import (
	"strings"
	"testing"
)

func TestSMT2MarshalOrdersUninterpretedSortAndFunctionDeclarations(t *testing.T) {
	_, manager, formula := marshalTestFormula(t, `(set-logic QF_UF)(declare-sort U 0)(declare-fun f (U) U)(declare-const x U)(assert (= (f x) x))`)
	text, ref, roundTrip := marshalTestRoundTrip(t, manager, formula)
	defer manager.DelRef(&ref)
	sortAt := strings.Index(text, "(declare-sort U)")
	functionAt := strings.Index(text, "(declare-fun f (U) U)")
	constantAt := strings.Index(text, "(declare-fun x () U)")
	if sortAt < 0 || functionAt < sortAt || constantAt < sortAt {
		t.Fatalf("dependency order mismatch:\n%s", text)
	}
	if roundTrip != formula {
		t.Fatalf("round trip changed formula: %v -> %v", formula, roundTrip)
	}
}
