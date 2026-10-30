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
