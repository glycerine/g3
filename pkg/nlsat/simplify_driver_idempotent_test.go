package nlsat

import "testing"

func TestSimplifyDriverSecondRunIsStable(t *testing.T) {
	s, _, _, _ := contentSimplificationFixture(t, true, AtomGT, 6, 5)
	first, err := s.Simplify()
	assignmentMust(t, err)
	second, err := s.Simplify()
	assignmentMust(t, err)
	if first.ContentRewritten == 0 || first.FourierMotzkin == 0 {
		t.Fatal("first run", first)
	}
	if second.LearnedRemoved != 0 || second.Subsumed != 0 || second.Unconstrained != 0 || second.ContentRewritten != 0 || second.FourierMotzkin != 0 || second.FactorsSplit != 0 || second.Rounds != 1 {
		t.Fatal("second run", second)
	}
}
