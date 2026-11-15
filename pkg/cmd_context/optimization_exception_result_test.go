package cmd_context

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
	"testing"
)

func TestOptimizationNonfatalExceptionBecomesUnknown(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	status, err := c.handleOptimizationFailure(z3exception.NewDefaultException("optimizer exception"))
	wantOptimizationCommandException(t, err, "optimizer exception")
	if status != lbool.Undef {
		t.Fatal(status)
	}
	reason, err := c.ReasonUnknown()
	contextMust(t, err)
	if reason != "optimizer exception" {
		t.Fatal(reason)
	}
}

// cmd_context::check_sat rethrows an optimize() z3_exception as a
// cmd_exception with ex.msg(); the check-sat result is still unknown.
func wantOptimizationCommandException(t *testing.T, err error, message string) {
	t.Helper()
	var source z3exception.Z3Exception
	if err == nil || !errors.As(err, &source) || source.Message() != message {
		t.Fatalf("want command exception %q, got %v", message, err)
	}
}

// opt_frontend catches the z3_exception itself: no command error.
func TestOptimizationFrontendExceptionBecomesUnknown(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	c.SetOptimizationFrontend()
	status, err := c.handleOptimizationFailure(z3exception.NewDefaultException("optimizer exception"))
	contextMust(t, err)
	if status != lbool.Undef {
		t.Fatal(status)
	}
}
