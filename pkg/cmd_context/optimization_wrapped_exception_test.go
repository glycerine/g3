package cmd_context

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestOptimizationWrappedNonfatalExceptionBecomesUnknown(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	cause := fmt.Errorf("optimizer stage: %w", z3exception.NewDefaultException("inner failure"))
	_, err = c.handleOptimizationFailure(cause)
	contextMust(t, err)
	reason, err := c.ReasonUnknown()
	contextMust(t, err)
	if reason != "inner failure" {
		t.Fatal(reason)
	}
}
