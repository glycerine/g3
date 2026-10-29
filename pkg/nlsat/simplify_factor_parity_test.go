package nlsat

import "testing"

func TestSimplifyUnitFactorsPreservesIndependentFactorParity(t *testing.T) {
	s, p, q, _ := factorSimplificationFixture(t, AtomGT, false, [2]bool{true, false}, Assumption{})
	count, err := s.SimplifyUnitFactors()
	assignmentMust(t, err)
	if count != 1 {
		t.Fatal(count)
	}
	factorWant(t, factorClauseSignatures(t, s, p, q), []string{"p2:gt,q:lt", "p2:lt,q:gt"})
}
