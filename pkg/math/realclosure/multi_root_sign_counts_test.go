package realclosure

import "testing"

func TestMultiRootPolynomialSignCounts(t *testing.T) {
	fixture := newMultiRootExactFixture(t, true)
	counts, err := fixture.f.store.countMultiRootPolynomialSigns(fixture.roots[0].root, fixture.values[0].functionNumerator.coefficients, true)
	if err != nil {
		t.Fatal(err)
	}
	if counts.equal != 1 || counts.positive != 1 || counts.negative != 1 || !counts.useSquare || len(counts.square) == 0 {
		t.Fatal(counts.equal, counts.positive, counts.negative, counts.useSquare, len(counts.square))
	}
	if err := fixture.f.store.releaseSturmCoefficients(counts.square); err != nil {
		t.Fatal(err)
	}
}
