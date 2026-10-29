package nlsat

import "testing"

func TestSimplifyLiteralContentIntegerLessRoundsConstantDown(t *testing.T) {
	s, _, _, _ := contentSimplificationFixture(t, true, AtomLT, 6, 5)
	_, err := s.SimplifyLiteralContent()
	assignmentMust(t, err)
	clause := s.clauses.originals[len(s.clauses.originals)-1]
	contentWantPolynomial(t, s, contentReplacementPolynomial(t, s, clause), 1, 0)
}

func TestSimplifyLiteralContentIntegerLessRoundsNegativeConstantDown(t *testing.T) {
	s, _, _, _ := contentSimplificationFixture(t, true, AtomLT, 6, -5)
	_, err := s.SimplifyLiteralContent()
	assignmentMust(t, err)
	clause := s.clauses.originals[len(s.clauses.originals)-1]
	contentWantPolynomial(t, s, contentReplacementPolynomial(t, s, clause), 1, -1)
}
