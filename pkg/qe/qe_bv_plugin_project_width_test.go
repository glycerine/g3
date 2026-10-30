package qe

import "testing"

func TestBitVectorPluginProjectModelPreservesWidths(t *testing.T) {
	for _, width := range []uint32{1, 7, 13} {
		f := newBVModelProjectFixture(t, width, 1)
		result := f.project(t)
		value := f.rational("0")
		var got uint32
		matched, err := f.manager.BitVectorNumeralValue(result, value, &got)
		fmMust(t, err)
		if !matched || got != width {
			t.Fatalf("width %d: matched=%v got=%d", width, matched, got)
		}
	}
}
