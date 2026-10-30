package cmd_context

import "testing"

func TestOptimizationModelBeforeCheckReturnsOwnedEmptyModel(t *testing.T) {
	c := contextFixture(t)
	model, err := c.OptimizationModel()
	contextMust(t, err)
	if count, countErr := model.NumDeclarations(); countErr != nil || count != 0 {
		t.Fatal(count, countErr)
	}
	contextMust(t, model.DecRef())
}
