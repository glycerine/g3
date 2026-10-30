package cmd_context

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestOptimizationModelAfterUnsatReturnsOwnedEmptyModel(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	falsity, err := m.MkFalse()
	contextMust(t, err)
	contextMust(t, c.Assert(falsity, ""))
	zero, err := m.ArithmeticIntegerNumeralText("0")
	contextMust(t, err)
	contextMust(t, c.AddObjective(zero, false))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.False {
		t.Fatal(status)
	}
	model, err := c.OptimizationModel()
	contextMust(t, err)
	if count, countErr := model.NumDeclarations(); countErr != nil || count != 0 {
		t.Fatal(count, countErr)
	}
	contextMust(t, model.DecRef())
}
