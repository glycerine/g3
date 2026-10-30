package sat

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestSATExtensionDecisionPhaseOverridesConfiguredPhase(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	extension := &satExtensionBindingFixture{phase: lbool.False}
	satOK(t, s.SetExtension(extension))
	_, err := s.MkVar(false, true)
	satOK(t, err)
	ok, err := s.decision.decide()
	satOK(t, err)
	if !ok || len(extension.events) < 2 || extension.events[0] != "push" || extension.events[1] != "phase" {
		t.Fatal(ok, extension.events)
	}
	value, err := s.sourceValueVar(0)
	satOK(t, err)
	if value != lbool.False {
		t.Fatal(value)
	}
}
