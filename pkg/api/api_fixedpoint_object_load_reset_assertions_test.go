package api

import "testing"

func TestAPIFixedpointObjectLoadSMT2AppendsAssertionsAcrossInputs(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.LoadSMT2String(`(assert true)`))
	astTestMust(t, f.fp.LoadSMT2String(`(reset-assertions) (assert false)`))
	assertions, err := f.fp.Assertions()
	astTestMust(t, err)
	if len(assertions) != 2 {
		t.Fatal("monotone fixedpoint background", len(assertions))
	}
	first, err := f.m.RefGet(assertions[0])
	astTestMust(t, err)
	second, err := f.m.RefGet(assertions[1])
	astTestMust(t, err)
	if !f.m.IsTrue(first) || !f.m.IsFalse(second) {
		t.Fatal("appended fixedpoint background changed")
	}
	releaseAPIRefs(t, f.m, assertions)
}

func TestAPIFixedpointObjectLoadSMT2ResetAssertionsWithinInput(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.LoadSMT2String(`(assert true) (reset-assertions) (assert false)`))
	assertions, err := f.fp.Assertions()
	astTestMust(t, err)
	if len(assertions) != 1 {
		t.Fatal("reset fixedpoint background", len(assertions))
	}
	handle, err := f.m.RefGet(assertions[0])
	astTestMust(t, err)
	if !f.m.IsFalse(handle) {
		t.Fatal("reset background is not false")
	}
	releaseAPIRefs(t, f.m, assertions)
}
