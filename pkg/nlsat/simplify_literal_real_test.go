package nlsat

import "testing"

func TestSimplifyLiteralContentRealStrictRelationUsesExactContent(t *testing.T) {
	s, _, old, oldClause := contentSimplificationFixture(t, false, AtomGT, 6, 5)
	count, err := s.SimplifyLiteralContent()
	assignmentMust(t, err)
	if count != 0 || s.clauses.owned[oldClause] == nil || old.Variable() != oldClause.Literals()[0].Variable() {
		t.Fatal("real relation changed", count)
	}

	s2, _, _, _ := contentSimplificationFixture(t, false, AtomLT, 6, 12)
	count, err = s2.SimplifyLiteralContent()
	assignmentMust(t, err)
	if count != 1 {
		t.Fatal("exact real content", count)
	}
	clause := s2.clauses.originals[len(s2.clauses.originals)-1]
	contentWantPolynomial(t, s2, contentReplacementPolynomial(t, s2, clause), 1, 2)
}
