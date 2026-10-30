package sat

import "testing"

func TestSATExtensionSimplifyProblemNotificationOrder(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	extension := &satExtensionFixture{}
	satOK(t, s.SetExtension(extension))
	satOK(t, s.simplifyProblem())
	n := len(extension.events)
	if n < 2 || extension.events[n-2] != "clauses" || extension.events[n-1] != "simplify" {
		t.Fatal(extension.events)
	}
}
