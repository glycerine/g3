package cmd_context

import (
	"strings"
	"testing"
)

func TestBuiltinTacticRegistryDrivesFactoryValidation(t *testing.T) {
	c := contextFixture(t)
	for _, name := range []string{"qe", "qe-sat", "skip", "fail", "simplify", "propagate-bv-bounds", "propagate-bv-bounds-new", "nla2bv"} {
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

func TestQETacticFactoryAllocationFailureReturnsNil(t *testing.T) {
	c := contextFixture(t)
	manager, err := c.Manager()
	contextMust(t, err)
	manager.Budget().FailAfter(0)
	created, err := c.NewBuiltinTactic("qe")
	manager.Budget().FailAfter(-1)
	if err == nil || created != nil {
		t.Fatal("failed QE factory returned a non-nil tactic", created, err)
	}
}
