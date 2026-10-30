package api

import "testing"

func TestAPIFixedpointObjectLoadSMT2ResetAssertions(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.LoadSMT2String(`(assert true)`))
	astTestMust(t, f.fp.LoadSMT2String(`(reset-assertions) (assert false)`))
	assertions, err := f.fp.Assertions()
	astTestMust(t, err)
	if len(assertions) != 1 {
		t.Fatal("monotone fixedpoint background", len(assertions))
	}
	handle, err := f.m.RefGet(assertions[0])
	astTestMust(t, err)
	if !f.m.IsFalse(handle) {
		t.Fatal("replacement background is not false")
	}
	releaseAPIRefs(t, f.m, assertions)
}
