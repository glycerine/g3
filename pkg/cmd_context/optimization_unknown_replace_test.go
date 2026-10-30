package cmd_context

import "testing"

func TestPublishOptimizationUnknownReplacesPriorOwnedResult(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	_, err = c.publishOptimizationUnknown("first")
	contextMust(t, err)
	contextMust(t, c.clearCheckSatResult())
	_, err = c.publishOptimizationUnknown("second")
	contextMust(t, err)
	reason, err := c.ReasonUnknown()
	contextMust(t, err)
	if reason != "second" {
		t.Fatal(reason)
	}
}
