package cmd_context

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/z3exception"
	"testing"
)

func TestOptimizationFatalAndOrdinaryErrorsPropagate(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	fatal := z3exception.MustZ3Error(z3exception.ErrorInternalFatal)
	if _, err = c.handleOptimizationFailure(fatal); !errors.Is(err, fatal) {
		t.Fatal(err)
	}
	ordinary := errors.New("owner failure")
	if _, err = c.handleOptimizationFailure(ordinary); !errors.Is(err, ordinary) {
		t.Fatal(err)
	}
	if c.checked {
		t.Fatal("propagated failure published a result")
	}
}
