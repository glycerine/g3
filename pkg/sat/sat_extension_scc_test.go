package sat

import "testing"

func TestSATExtensionSCCNoChangeDoesNotNotify(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	extension := &satExtensionFixture{}
	satOK(t, s.SetExtension(extension))
	count, err := s.SCCBinary()
	satOK(t, err)
	if count != 0 || len(extension.events) != 0 {
		t.Fatal(count, extension.events)
	}
}
