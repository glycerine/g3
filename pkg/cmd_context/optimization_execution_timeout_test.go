package cmd_context

import (
	"testing"
	"time"
)

func TestOptimizationExecutionTimerCancelsAndCloseRestores(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	contextMust(t, c.SetUIntOption("timeout", 1))
	scope, err := c.beginOptimizationExecution()
	contextMust(t, err)
	deadline := time.Now().Add(100 * time.Millisecond)
	for !c.manager.ResourceLimit().CancelFlag() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !c.manager.ResourceLimit().CancelFlag() {
		t.Fatal("optimization timer did not cancel")
	}
	contextMust(t, scope.Close())
	if c.manager.ResourceLimit().CancelFlag() {
		t.Fatal("timer cancellation leaked after close")
	}
}
