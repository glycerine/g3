package cmd_context

import (
	"math/big"
	"testing"
)

func TestOptimizationAPISnapshotsObjectivesAndCopiesWeights(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	boolean, err := m.MkBoolSort()
	contextMust(t, err)
	name, err := m.Symbols.Intern("optimization-api-p")
	contextMust(t, err)
	p, err := m.MkConst(name, boolean)
	contextMust(t, err)
	weight := big.NewRat(3, 2)
	index, err := c.AddSoftConstraintIndexed(p, weight, "group")
	contextMust(t, err)
	if index != 0 {
		t.Fatal("soft objective index", index)
	}
	objective, err := c.Objective(index)
	contextMust(t, err)
	if !objective.Soft || objective.ID != "group" || len(objective.SoftTerms) != 1 || objective.SoftTerms[0].Expression != p || objective.SoftTerms[0].Weight.Cmp(weight) != 0 {
		t.Fatal("soft objective snapshot", objective)
	}
	weight.SetInt64(9)
	if objective.SoftTerms[0].Weight.Cmp(big.NewRat(3, 2)) != 0 {
		t.Fatal("snapshot weight aliases caller")
	}
}
