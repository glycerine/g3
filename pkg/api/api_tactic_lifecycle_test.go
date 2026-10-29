package api

import (
	"errors"
	"testing"
)

func TestAPITacticConstructorsAndLifecycle(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	constructors := []struct {
		name string
		make func() (*APITactic, error)
	}{
		{"skip", f.c.SkipTactic},
		{"fail", f.c.FailTactic},
		{"fail-if-undecided", f.c.FailIfUndecidedTactic},
		{"builtin-simplify", func() (*APITactic, error) { return f.c.Tactic("simplify") }},
	}
	for _, test := range constructors {
		tactic, err := test.make()
		astTestMust(t, err)
		astTestMust(t, tactic.Close())
		if err = tactic.open(); !errors.Is(err, ErrSMTContextOwner) {
			t.Fatalf("%s remained open: %v", test.name, err)
		}
		astTestMust(t, tactic.Close())
	}
	if _, err := f.c.Tactic("not-a-retained-tactic"); err == nil {
		t.Fatal("unknown tactic accepted")
	}
}
