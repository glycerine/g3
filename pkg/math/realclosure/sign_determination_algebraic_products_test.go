package realclosure

import "testing"

func TestSignDeterminationExpansionConsumesAlgebraicPolynomialProduct(t *testing.T) {
	f := newRealFunctionFixture(t)
	state := newSignDetermination(2)
	mustRealFunction(t, state.attachInitialPolynomial(f.store, f.one))
	t.Cleanup(func() { mustRealFunction(t, state.closePolynomials()) })
	// [[1,1],[0,1]] * [1,1] = [2,1]. Both sign columns survive.
	kept, allOne, err := state.advanceRationalProducts(1, 1, 0, []*realValue{f.alpha}, []int{2, 1})
	mustRealFunction(t, err)
	if !kept || !allOne || len(state.prs) != 2 || len(state.qs) != 1 {
		t.Fatal("algebraic sign expansion state", kept, allOne, len(state.prs), len(state.qs))
	}
	found := false
	for _, row := range state.prs {
		for _, coefficient := range row.coefficients {
			if coefficient != nil && !coefficient.isRational && coefficient.root == f.root {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("prs*q did not retain the algebraic coefficient")
	}
}
