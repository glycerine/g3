package realclosure

import "testing"

func TestSignDeterminationThreeWayExpansionBuildsReducedAlgebraicSquare(t *testing.T) {
	f := newRealFunctionFixture(t)
	state := newSignDetermination(3)
	mustRealFunction(t, state.attachInitialPolynomial(f.store, f.one))
	t.Cleanup(func() { mustRealFunction(t, state.closePolynomials()) })
	kept, allOne, err := state.advanceAlgebraicProducts(1, 1, 1, []*realValue{f.alpha}, []int{3, 0, 2})
	mustRealFunction(t, err)
	if !kept || !allOne || len(state.prs) != 3 || len(state.qs) != 1 {
		t.Fatal("three-way algebraic expansion", kept, allOne, len(state.prs), len(state.qs))
	}
	foundSquare := false
	for _, row := range state.prs {
		if len(row.coefficients) == 1 && row.coefficients[0] != nil && row.coefficients[0].isRational && f.coefficientText(row.coefficients[0]) == "2" {
			foundSquare = true
		}
	}
	if !foundSquare {
		t.Fatal("q squared row was not reduced modulo the algebraic definition")
	}
}
