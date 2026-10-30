package sat

import (
	"errors"
	"testing"
)

func TestSATExtensionCallbackErrorsPreservePublishedPrefixes(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	want := errors.New("extension stop")
	extension := &satExtensionBindingFixture{err: want}
	satOK(t, s.SetExtension(extension))
	if err := s.Lifecycle.push(); !errors.Is(err, want) || s.Variables.scopeLevel != 1 {
		t.Fatal(err, s.Variables.scopeLevel)
	}
}
