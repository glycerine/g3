package sat

import (
	"errors"
	"testing"
)

func TestSATExtensionClauseNotificationErrorPropagates(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	want := errors.New("extension clauses stop")
	extension := &satExtensionFixture{err: want}
	satOK(t, s.SetExtension(extension))
	if err := s.propagation.notifyExtensionClausesModified(); !errors.Is(err, want) {
		t.Fatal(err)
	}
}
