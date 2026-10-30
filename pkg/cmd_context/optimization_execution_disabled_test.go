package cmd_context

import (
	"testing"
	"time"
)

func TestOptimizationExecutionZeroTimeoutIsDisabled(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	contextMust(t, c.SetUIntOption("timeout", 0))
	scope, err := c.beginOptimizationExecution()
	contextMust(t, err)
	time.Sleep(2 * time.Millisecond)
	if c.manager.ResourceLimit().CancelFlag() {
		t.Fatal("disabled timer canceled resource limit")
	}
	contextMust(t, scope.Close())
}
