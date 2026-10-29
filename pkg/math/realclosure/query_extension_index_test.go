package realclosure

import "testing"

func TestAlgebraicMetadataExtensionAndDefiningCoefficients(t *testing.T) {
	f := newRankedValueFixture(t, true)
	index, err := f.base.store.extensionIndex(f.alpha)
	if err != nil || index != f.alpha.root.index {
		t.Fatal(index, err)
	}
	if rationalIndex, err := f.base.store.extensionIndex(f.one); err != nil || rationalIndex != -1 {
		t.Fatal(rationalIndex, err)
	}
	count, err := f.base.store.numDefiningCoefficients(f.alpha)
	if err != nil || count != len(f.alpha.root.polynomial.coefficients) {
		t.Fatal(count, err)
	}
}
