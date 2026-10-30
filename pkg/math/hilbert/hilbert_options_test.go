package hilbert

import "testing"

func TestSourceConfigurationSetters(t *testing.T) {
	b := New(nil)
	b.SetUseSupport(false)
	b.SetUseOrderedSupport(false)
	b.SetUseOrderedSubsumption(false)
	if b.useSupport || b.orderedSupport || b.orderedSubsumption {
		t.Fatal("configuration setter did not update source option")
	}
	b.SetUseSupport(true)
	b.SetUseOrderedSupport(true)
	b.SetUseOrderedSubsumption(true)
	if !b.useSupport || !b.orderedSupport || !b.orderedSubsumption {
		t.Fatal("configuration setter did not restore source option")
	}
}
