package cmd_context

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/lbool"
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

func TestOptimizationUnknownReasonUsesBackendFailure(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	reason, err := c.optimizationResultUnknownReason(&opt.ArithmeticResult{ReasonUnknown: "(incomplete quantifiers)"})
	contextMust(t, err)
	if reason != "(incomplete quantifiers)" {
		t.Fatal("optimization backend reason", reason)
	}
}

func TestOptimizationCancellationOverridesBackendFailure(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	c.manager.ResourceLimit().Cancel()
	defer c.manager.ResourceLimit().ResetCancel()
	reason, err := c.optimizationResultUnknownReason(&opt.ArithmeticResult{ReasonUnknown: "(incomplete quantifiers)"})
	contextMust(t, err)
	if want := c.manager.ResourceLimit().CancelMessage(); reason != want {
		t.Fatal("optimization cancellation reason", reason, "want", want)
	}
}
