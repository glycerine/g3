package sat

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// satContinueOnceExtension answers CR_CONTINUE once, then CR_DONE.
type satContinueOnceExtension struct {
	satExtensionFixture
	checks int
}

func (e *satContinueOnceExtension) Check() (SATExtensionCheckResult, error) {
	e.checks++
	if e.checks == 1 {
		return SATExtensionContinue, nil
	}
	return SATExtensionDone, nil
}

// AUDIT.md sat_04-F3: bounded_search goes on searching when final_check
// returns l_undef; it is not a restart boundary.
func TestSATBoundedSearchContinuesAfterExtensionContinue(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false, true)
		satOK(t, err)
	}
	satOK(t, s.MkClause([]Literal{MakeLiteral(0, false), MakeLiteral(1, false)}))
	extension := &satContinueOnceExtension{}
	satOK(t, s.SetExtension(extension))
	r, err := s.Check(nil)
	satOK(t, err)
	if r != lbool.True || extension.checks != 2 {
		t.Fatal(r, extension.checks)
	}
	// No restart, so no simplify_problem and no m_ext->simplify().
	for _, event := range extension.events {
		if event == "simplify" {
			t.Fatal("CR_CONTINUE restarted and simplified", extension.events)
		}
	}
}
