package api

import "testing"

func TestSMTContextUpdateQuantifierWeightOwnsRebuiltValue(t *testing.T) {
	f := newQuantifierUpdateFixture(t)
	ref, err := f.context.UpdateQuantifierWeight(f.value, -13)
	updated := f.own(t, ref, err)
	if weight, err := f.manager.GetQuantifierWeight(updated); err != nil || weight != -13 {
		t.Fatal(weight, err)
	}
	if weight, err := f.manager.GetQuantifierWeight(f.value); err != nil || weight != 7 {
		t.Fatal("source mutated", weight, err)
	}
	qid, skid, err := f.context.QuantifierIdentifiers(updated)
	if err != nil || qid != "source-qid" || skid != "source-skid" {
		t.Fatal(qid, skid, err)
	}
}
