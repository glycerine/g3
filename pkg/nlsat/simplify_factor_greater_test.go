package nlsat

import "testing"

func TestSimplifyUnitFactorsGreaterThanPolarities(t *testing.T) {
	for _, test := range []struct {
		negative bool
		want     []string
	}{
		{false, []string{"p:gt,q:lt", "p:lt,q:gt"}},
		{true, []string{"!p:gt,!q:gt", "!p:lt,!q:lt"}},
	} {
		s, p, q, _ := factorSimplificationFixture(t, AtomGT, test.negative, [2]bool{}, Assumption{})
		count, err := s.SimplifyUnitFactors()
		assignmentMust(t, err)
		if count != 1 {
			t.Fatal("greater-than split count", count)
		}
		factorWant(t, factorClauseSignatures(t, s, p, q), test.want)
	}
}
