package nlsat

import "testing"

func TestSimplifyDriverNormalizesContentBeforeFourierMotzkin(t *testing.T) {
	s, _, _, _ := contentSimplificationFixture(t, true, AtomGT, 6, 5)
	summary, err := s.Simplify()
	assignmentMust(t, err)
	if summary.ContentRewritten != 1 || summary.FourierMotzkin != 1 {
		t.Fatal("phase counts", summary)
	}
	if len(s.fmBounds) != 1 {
		t.Fatal("missing normalized bound", len(s.fmBounds))
	}
}
