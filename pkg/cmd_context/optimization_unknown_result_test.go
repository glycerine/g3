package cmd_context

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestPublishOptimizationUnknownOwnsStatusAndReason(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	status, err := c.publishOptimizationUnknown("optimizer stopped")
	contextMust(t, err)
	if status != lbool.Undef || !c.checked || c.status != lbool.Undef {
		t.Fatal(status, c.checked, c.status)
	}
	reason, err := c.ReasonUnknown()
	contextMust(t, err)
	if reason != "optimizer stopped" {
		t.Fatal(reason)
	}
}
