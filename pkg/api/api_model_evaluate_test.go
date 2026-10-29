package api

import "testing"

func TestAPIModelEvaluateRegisteredConstant(t *testing.T) {
	f := newAPIQEBooleanFixture(t)
	value, err := f.model.Evaluate(f.x, false)
	astTestMust(t, err)
	got, err := f.m.RefGet(value)
	astTestMust(t, err)
	if got != f.yes {
		t.Fatal("evaluated constant", got, f.yes)
	}
	astTestMust(t, f.m.DelRef(&value))
}
