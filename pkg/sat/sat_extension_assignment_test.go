package sat

import "testing"

func TestSATExtensionReceivesExternalAssignment(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	extension := &satExtensionBindingFixture{}
	satOK(t, s.SetExtension(extension))
	v, err := s.MkVar(true, true)
	satOK(t, err)
	satOK(t, s.propagation.assign(MakeLiteral(v, false), satJustification{}))
	if len(extension.events) != 1 || extension.events[0] != "asserted" {
		t.Fatal(extension.events)
	}
}
