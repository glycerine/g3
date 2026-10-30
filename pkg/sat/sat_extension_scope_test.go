package sat

import "testing"

func TestSATExtensionScopeCallbacksPreserveSourceOrder(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	extension := &satExtensionBindingFixture{}
	satOK(t, s.SetExtension(extension))
	satOK(t, s.Lifecycle.push())
	if s.Variables.scopeLevel != 1 || len(extension.events) != 1 || extension.events[0] != "push" {
		t.Fatal(s.Variables.scopeLevel, extension.events)
	}
	satOK(t, s.Lifecycle.pop(1))
	if s.Variables.scopeLevel != 0 || len(extension.events) != 2 || extension.events[1] != "pop" {
		t.Fatal(s.Variables.scopeLevel, extension.events)
	}
}
