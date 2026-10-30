package api

import "testing"

func TestFloatingPointCustomSortDimensions(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	sort, err := f.context.FloatingPointSort(17, 65)
	sort = f.pin(t, sort, err)
	if bits, err := f.context.FloatingPointExponentBits(sort); err != nil || bits != 17 {
		t.Fatal(bits, err)
	}
	if bits, err := f.context.FloatingPointSignificandBits(sort); err != nil || bits != 65 {
		t.Fatal(bits, err)
	}
}
