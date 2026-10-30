package cmd_context

import "testing"

func TestOptimizationExecutionScopeRestoresResourceLimit(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	contextMust(t, c.SetUIntOption("rlimit", 17))
	limit := c.manager.ResourceLimit()
	before := limit.Limit
	scope, err := c.beginOptimizationExecution()
	contextMust(t, err)
	if limit.Limit == before {
		t.Fatal("resource limit was not scoped", before)
	}
	contextMust(t, scope.Close())
	if limit.Limit != before || limit.CancelFlag() {
		t.Fatal("resource state leaked", limit.Limit, before, limit.CancelFlag())
	}
	contextMust(t, scope.Close())
}
