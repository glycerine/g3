package cmd_context

import (
	"github.com/glycerine/g3/pkg/util/numeral"
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
	q := m.NumeralService().Manager
	weight := numeral.NewRational()
	contextMust(t, q.SetString(&weight, "3/2"))
	defer func() { contextMust(t, q.Del(&weight)) }()
	index, err := c.AddSoftConstraintIndexed(p, &weight, "group")
	contextMust(t, err)
	if index != 0 {
		t.Fatal("soft objective index", index)
	}
	objective, err := c.Objective(index)
	contextMust(t, err)
	defer func() { contextMust(t, objective.Release(q)) }()
	if !objective.Soft || objective.ID != "group" || len(objective.SoftTerms) != 1 || objective.SoftTerms[0].Expression != p {
		t.Fatal("soft objective snapshot", objective)
	}
	if text, e := q.String(&objective.SoftTerms[0].Weight); e != nil || text != "3/2" {
		t.Fatal("soft objective weight", text, e)
	}
	contextMust(t, q.SetInt32(&weight, 9))
	if text, e := q.String(&objective.SoftTerms[0].Weight); e != nil || text != "3/2" {
		t.Fatal("snapshot weight aliases caller", text, e)
	}
}
