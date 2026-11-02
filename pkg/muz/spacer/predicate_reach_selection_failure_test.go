package spacer

import (
	"testing"

	"github.com/glycerine/g3/pkg/model"
)

func TestSpacerOriginReachSelectionAllocationFailureAndRetry(t *testing.T) {
	complete, failures := false, 0
	for ordinal := int64(0); ordinal < 128 && !complete; ordinal++ {
		f := newContextAnswerFixture(t, false)
		p, m := f.context.transformers[f.query], f.context.manager
		if len(p.reachCaseVariables) == 0 {
			t.Fatal("missing reach cases")
		}
		md, err := model.NewModel(m)
		depthMust(t, err)
		current, err := m.RefGet(p.reachCaseVariables[0])
		depthMust(t, err)
		shifted, err := p.owner.FormulaNewToOld(current, 2, false)
		depthMust(t, err)
		h, err := m.RefGet(shifted)
		depthMust(t, err)
		decl, err := m.GetDecl(h)
		depthMust(t, err)
		falsity, err := m.MkFalse()
		depthMust(t, err)
		depthMust(t, md.Core().RegisterConstant(decl, falsity))
		before := m.Stats().LiveReferences
		m.Budget().FailAfter(ordinal)
		fact, found, failure := p.UsedOriginReachFact(md.Core(), 2)
		m.Budget().FailAfter(-1)
		if failure != nil {
			failures++
			if found || fact != nil {
				t.Fatal("failed selection published a fact", ordinal, failure)
			}
		} else {
			complete = true
			if !found || fact == nil {
				t.Fatal("selection did not find interpreted case", ordinal)
			}
			depthMust(t, fact.Close())
		}
		if m.Stats().LiveReferences != before {
			t.Fatal("selection failure leaked references", ordinal, failure)
		}
		fact, found, err = p.UsedOriginReachFact(md.Core(), 2)
		depthMust(t, err)
		if !found || fact == nil {
			t.Fatal("selection retry lost its case", ordinal)
		}
		depthMust(t, fact.Close())
		depthMust(t, m.DelRef(&shifted))
		depthMust(t, md.Close())
	}
	if !complete || failures == 0 {
		t.Fatal("selection allocation campaign incomplete", complete, failures)
	}
}
