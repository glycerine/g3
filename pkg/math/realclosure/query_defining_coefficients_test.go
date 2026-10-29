package realclosure

import "testing"

func TestAlgebraicMetadataDefiningCoefficientOwnership(t *testing.T) {
	f := newRankedValueFixture(t, true)
	for index, expected := range f.alpha.root.polynomial.coefficients {
		before := uint32(0)
		if expected != nil {
			before = expected.refs
		}
		coefficient, err := f.base.store.definingCoefficient(f.alpha, index)
		if err != nil || coefficient != expected {
			t.Fatal(index, coefficient, expected, err)
		}
		if expected != nil && expected.refs != before+1 {
			t.Fatal("coefficient not retained", index, before, expected.refs)
		}
		f.releaseValues(coefficient)
	}
	if coefficient, err := f.base.store.definingCoefficient(f.alpha, 99); err != nil || coefficient != nil {
		t.Fatal(coefficient, err)
	}
}
