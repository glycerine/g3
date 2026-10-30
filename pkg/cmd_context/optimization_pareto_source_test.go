package cmd_context

import (
	"testing"

	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestParetoModelEvaluationLeavesUninterpretedObjectiveIncomplete(t *testing.T) {
	c := contextFixture(t)
	x := optimizationComparisonInteger(t, c, "pareto-uninterpreted-objective")
	md, err := model.NewModelCore(c.manager)
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, md.Close()) })
	c.objectives = []optimizationObjective{{expression: x, maximize: true}}
	t.Cleanup(func() { c.objectives = nil })
	values, err := c.evaluateParetoModel(md)
	contextMust(t, err)
	if len(values) != 1 || values[0].comparable {
		t.Fatal("uninterpreted objective was completed", values)
	}
}

func TestParetoIncomparableObjectiveUsesNeutralComparison(t *testing.T) {
	c := contextFixture(t)
	x := optimizationComparisonInteger(t, c, "pareto-neutral-objective")
	c.objectives = []optimizationObjective{{expression: x, maximize: true}}
	t.Cleanup(func() { c.objectives = nil })
	for _, dominates := range []bool{true, false} {
		backend, err := c.newParetoBackend(nil)
		contextMust(t, err)
		values := []paretoModelValue{{comparable: false}}
		contextMust(t, c.assertParetoFormula(backend, values, dominates))
		status, err := backend.CheckSat(nil)
		contextMust(t, err)
		if status != lbool.False {
			t.Fatal("neutral Pareto comparison status", dominates, status)
		}
		contextMust(t, backend.Close())
	}
}
