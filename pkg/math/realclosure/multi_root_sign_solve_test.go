package realclosure

import "testing"

func TestCompletedSignExtensionSolveAndSelection(t *testing.T) {
	fixture := newMultiRootExactFixture(t, false)
	root := fixture.roots[0].root
	counts, err := fixture.f.store.countMultiRootPolynomialSigns(root, fixture.values[0].functionNumerator.coefficients, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fixture.f.store.releaseSturmCoefficients(counts.square) }()
	expanded, products, err := fixture.f.store.expandCompletedSignQueries(root, fixture.values[0].functionNumerator.coefficients, counts.square, counts.useSquare, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fixture.f.store.releaseMultiRootProducts(products) }()
	matrix, cardinalities, qMatrix, err := solveCompletedSignExtension(root.signs, counts, expanded)
	if err != nil || matrix.cols != 9 || len(cardinalities) != 9 || qMatrix.cols != 3 {
		t.Fatal(matrix.cols, len(cardinalities), qMatrix.cols, err)
	}
	for _, rootValue := range fixture.roots {
		if _, err := selectExtendedRootSign(rootValue.root, cardinalities, &qMatrix); err != nil {
			t.Fatal(err)
		}
	}
}
