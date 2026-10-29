package smt2

import "testing"

func TestSMT2MarshalQuantifierAndArithmeticRoundTrip(t *testing.T) {
	_, manager, formula := marshalTestFormula(t, `(set-logic AUFLIA)(declare-fun a (Int) Int)(assert (forall ((x Int)) (! (= (a x) (+ x 1)) :qid q)))`)
	_, ref, roundTrip := marshalTestRoundTrip(t, manager, formula)
	defer func() {
		if err := manager.DelRef(&ref); err != nil {
			t.Error(err)
		}
	}()
	if roundTrip != formula {
		t.Fatalf("quantified round trip changed handle: %v -> %v", formula, roundTrip)
	}
	wellSorted, err := manager.IsWellSorted(roundTrip, nil)
	if err != nil || !wellSorted {
		t.Fatalf("well sorted %t, %v", wellSorted, err)
	}
}
