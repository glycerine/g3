package sat

import (
	"errors"
	"testing"
)

func TestSATExtensionBindingAndMissingBindingGuard(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	s.propagation.extensionPresent = true
	if _, err := s.propagation.sourceExtension(); !errors.Is(err, ErrSATMissingSourceExtBinding) {
		t.Fatal(err)
	}
	extension := &satExtensionBindingFixture{}
	satOK(t, s.SetExtension(extension))
	if got, err := s.propagation.sourceExtension(); err != nil || got != extension {
		t.Fatal(got, err)
	}
	satOK(t, s.SetExtension(nil))
	if s.propagation.extensionPresent {
		t.Fatal("nil extension remained bound")
	}
}
