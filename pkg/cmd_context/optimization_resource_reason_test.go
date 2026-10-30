package cmd_context

import (
	"errors"
	"testing"
)

func TestOptimizationResourceLimitUsesSourceCancellationReason(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	limit := c.manager.ResourceLimit()
	contextMust(t, limit.Push(1))
	defer limit.Pop()
	if !limit.Inc() || limit.Inc() {
		t.Fatal("failed to reach scoped resource limit")
	}
	_, err = c.handleOptimizationFailure(errors.New("backend detail"))
	contextMust(t, err)
	reason, err := c.ReasonUnknown()
	contextMust(t, err)
	if reason != "max. resource limit exceeded" {
		t.Fatal(reason)
	}
}
