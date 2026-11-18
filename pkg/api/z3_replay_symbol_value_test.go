package api

import "testing"

// AUDIT.md api_03-F8: a serialized $ || is symbol(""), not symbol::null.
func TestReplaySymbolValueEmptyIsNotNull(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	owner := &replayContextOwner{context: f.c, results: &apiContextResults{manager: f.m}}
	sym, err := replaySymbolValue(owner, z3ReplayValue{kind: replaySymbol, s: ""})
	if err != nil {
		t.Fatal(err)
	}
	if sym.IsNull() {
		t.Fatal("$ || replayed as symbol::null")
	}
	null, err := replaySymbolValue(owner, z3ReplayValue{kind: replaySymbol, nullSymbol: true})
	if err != nil || !null.IsNull() {
		t.Fatal("N is not symbol::null", null, err)
	}
}
