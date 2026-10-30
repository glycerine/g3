package cmd_context

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
	"testing"
)

func TestOptimizationNonfatalExceptionBecomesUnknown(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	status, err := c.handleOptimizationFailure(z3exception.NewDefaultException("optimizer exception"))
	contextMust(t, err)
	if status != lbool.Undef {
		t.Fatal(status)
	}
	reason, err := c.ReasonUnknown()
	contextMust(t, err)
	if reason != "optimizer exception" {
		t.Fatal(reason)
	}
}
