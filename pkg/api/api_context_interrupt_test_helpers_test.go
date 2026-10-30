package api

import "testing"

func newAPIInterruptFixture(t *testing.T) *SMTContext {
	t.Helper()
	context, err := NewSMTContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return context
}
