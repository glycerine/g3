package nlsat

import "testing"

func TestSimplifyLiteralContentIntegerGreaterRoundsConstantUp(t *testing.T) {
	s, _, _, _ := contentSimplificationFixture(t, true, AtomGT, 6, 5)
	count, err := s.SimplifyLiteralContent()
	assignmentMust(t, err)
	if count != 1 {
		t.Fatal("rewrite count", count)
	}
	clause := s.clauses.originals[len(s.clauses.originals)-1]
	contentWantPolynomial(t, s, contentReplacementPolynomial(t, s, clause), 1, 1)
}

func TestSimplifyLiteralContentIntegerGreaterRoundsNegativeConstantUp(t *testing.T) {
	s, _, _, _ := contentSimplificationFixture(t, true, AtomGT, 6, -5)
	_, err := s.SimplifyLiteralContent()
	assignmentMust(t, err)
	clause := s.clauses.originals[len(s.clauses.originals)-1]
	contentWantPolynomial(t, s, contentReplacementPolynomial(t, s, clause), 1, 0)
}
