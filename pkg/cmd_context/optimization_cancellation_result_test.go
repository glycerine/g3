package cmd_context

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestOptimizationCancellationBecomesUnknown(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	c.manager.ResourceLimit().Cancel()
	defer c.manager.ResourceLimit().ResetCancel()
	status, err := c.handleOptimizationFailure(errors.New("backend detail"))
	contextMust(t, err)
	if status != lbool.Undef {
		t.Fatal(status)
	}
	reason, err := c.ReasonUnknown()
	contextMust(t, err)
	if reason != "canceled" {
		t.Fatal(reason)
	}
}

func TestOptimizationUnknownReasonPreservesCancellation(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	c.manager.ResourceLimit().Cancel()
	defer c.manager.ResourceLimit().ResetCancel()
	reason, err := c.optimizationUnknownReason()
	contextMust(t, err)
	if want := c.manager.ResourceLimit().CancelMessage(); reason != want {
		t.Fatal("optimization cancellation reason", reason, "want", want)
	}
}
