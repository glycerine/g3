package cmd_context

import "testing"

func TestOptimizationEmptyModelsAreIndependentOwners(t *testing.T) {
	c := contextFixture(t)
	first, err := c.OptimizationModel()
	contextMust(t, err)
	second, err := c.OptimizationModel()
	contextMust(t, err)
	if first == second {
		t.Fatal("empty model fallback was shared")
	}
	contextMust(t, first.DecRef())
	if count, countErr := second.NumConstants(); countErr != nil || count != 0 {
		t.Fatal(count, countErr)
	}
	contextMust(t, second.DecRef())
}
