package realclosure

import "testing"

func TestNormalizedSignDeterminationAdvanceComputesAlgebraicTarskiGroups(t *testing.T) {
	f := newNormalizedSignQueryFixture(t)
	state := newSignDetermination(2)
	mustRealFunction(t, state.attachInitialPolynomial(f.store, f.one))
	t.Cleanup(func() { mustRealFunction(t, state.closePolynomials()) })
	kept, allOne, err := state.advanceNormalizedUnbounded(f.polynomial, f.x, []int{2, 0})
	mustRealFunction(t, err)
	if !kept || !allOne || len(state.prs) != 2 || len(state.qs) != 1 {
		t.Fatal("normalized sign determination state", kept, allOne, len(state.prs), len(state.qs))
	}
}
