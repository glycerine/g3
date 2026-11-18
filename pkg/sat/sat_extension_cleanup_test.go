package sat

import "testing"

// AUDIT.md sat_01-F5: solver::cleanup only notifies clauses_modifed.
func TestSATExtensionCleanupNotifiesWithoutSimplify(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	_, err := s.MkVar(false, true)
	satOK(t, err)
	satOK(t, s.MkClause([]Literal{MakeLiteral(0, false)}))
	extension := &satExtensionFixture{}
	satOK(t, s.SetExtension(extension))
	satOK(t, s.CleanupClauses())
	if len(extension.events) != 1 || extension.events[0] != "clauses" {
		t.Fatal(extension.events)
	}
}
