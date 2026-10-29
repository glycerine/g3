package nlsat

import "testing"

func TestSimplifyFourierMotzkinResetReleasesBoundOwnership(t *testing.T) {
	s, _, _ := fmIntervalFixture(t, false, 1, 3)
	_, _, err := s.SimplifyFourierMotzkin()
	assignmentMust(t, err)
	if len(s.fmBounds) != 2 {
		t.Fatal("bounds", len(s.fmBounds))
	}
	assignmentMust(t, s.Reset())
	if len(s.fmBounds) != 0 {
		t.Fatal("reset retained bounds")
	}
}
