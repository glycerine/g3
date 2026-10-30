package sat

import "testing"

func TestSATExtensionPublicSimplifyNotifiesAfterFinalize(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	extension := &satExtensionFixture{}
	satOK(t, s.SetExtension(extension))
	satOK(t, s.SimplifyClauses(false))
	if len(extension.events) != 1 || extension.events[0] != "clauses" {
		t.Fatal(extension.events)
	}
}
