package cmd_context

import (
	"strings"
	"testing"
)

func TestBuiltinTacticRegistryDrivesFactoryValidation(t *testing.T) {
	c := contextFixture(t)
	for _, name := range []string{"skip", "fail", "simplify", "propagate-bv-bounds", "propagate-bv-bounds-new"} {
		created, err := c.NewBuiltinTactic(name)
		contextMust(t, err)
		if created == nil {
			t.Fatal("nil factory", name)
		}
		contextMust(t, created.IncRef())
		contextMust(t, created.DecRef())
	}
	if _, err := c.NewBuiltinTactic("not-a-retained-tactic"); err == nil || !strings.Contains(err.Error(), "unknown tactic") {
		t.Fatal("unknown factory", err)
	}
}
