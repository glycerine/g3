package api

import "testing"

func TestAPIFixedpointObjectLoadSMT2ReportsParserFailure(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	if err := f.fp.LoadSMT2String(`(rule p`); err == nil {
		t.Fatal("malformed fixedpoint stream loaded")
	}
}
