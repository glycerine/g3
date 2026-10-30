package qe

import "testing"

func TestBitVectorPluginProjectModelSubstitutesConcreteValue(t *testing.T) {
	f := newBVModelProjectFixture(t, 8, 37)
	result := f.project(t)
	found, err := f.contains.Check(result)
	fmMust(t, err)
	if found {
		t.Fatal("model projection retained target")
	}
	value := f.rational("37")
	var width uint32
	matched, err := f.manager.BitVectorNumeralValue(result, value, &width)
	fmMust(t, err)
	if !matched || width != 8 {
		t.Fatalf("matched=%v width=%d", matched, width)
	}
}
