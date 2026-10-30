package qe

import "testing"

func TestBitVectorPluginPolicyCallbacks(t *testing.T) {
	f := newBVPluginFixture(t, 4)
	if weight, err := f.plugin.Weight(f.contains, f.formula); err != nil || weight != 2 {
		t.Fatalf("weight=%d err=%v", weight, err)
	}
	if err := f.plugin.Assign(f.contains, f.formula, f.rational("999")); err != nil {
		t.Fatal(err)
	}
	if solved, err := f.plugin.Solve(nil, f.formula); err != nil || solved {
		t.Fatalf("solved=%v err=%v", solved, err)
	}
	if _, handled, err := f.plugin.MkAtom(f.formula, true); err != nil || handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
}
