package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"

	"github.com/glycerine/g3/pkg/model"
)

func TestSpacerOriginReachSelectionAllocationFailureAndRetry(t *testing.T) {
	testSpacerReachSelectionFailures(t, true)
}
func TestSpacerCurrentReachSelectionAllocationFailureAndRetry(t *testing.T) {
	testSpacerReachSelectionFailures(t, false)
}

func testSpacerReachSelectionFailures(t *testing.T, origin bool) {
	t.Helper()
	complete, failures := false, 0
	for ordinal := int64(0); ordinal < 128 && !complete; ordinal++ {
		var budget *arena.Budget
		t.Cleanup(func() {
			if budget != nil && budget.Used() != 0 {
				t.Fatal("reach selection fixture leaked bytes", budget.Used())
			}
		})
		f := newContextAnswerFixture(t, false)
		p, m := f.context.transformers[f.query], f.context.manager
		budget = m.Budget()
		if len(p.reachCaseVariables) == 0 {
			t.Fatal("missing reach cases")
		}
		md, err := model.NewModel(m)
		depthMust(t, err)
		current, err := m.RefGet(p.reachCaseVariables[0])
		depthMust(t, err)
		var shifted ast.ASTRef
		h := current
		if origin {
			shifted, err = p.owner.FormulaNewToOld(current, 2, false)
			depthMust(t, err)
			h, err = m.RefGet(shifted)
			depthMust(t, err)
		}
		decl, err := m.GetDecl(h)
		depthMust(t, err)
		falsity, err := m.MkFalse()
		depthMust(t, err)
		depthMust(t, md.Core().RegisterConstant(decl, falsity))
		selectFact := func() (*ReachFact, bool, error) {
			if origin {
				return p.UsedOriginReachFact(md.Core(), 2)
			}
			return p.UsedReachFact(md.Core(), true)
		}
		before := m.Stats().LiveReferences
		caseCount, err := m.GetRefCount(h)
		depthMust(t, err)
		mode, err := m.ProofMode()
		depthMust(t, err)
		m.Budget().FailAfter(ordinal)
		fact, found, failure := selectFact()
		m.Budget().FailAfter(-1)
		if failure != nil {
			failures++
			var cleanup memory.ConstructionFailure
			if errors.As(failure, &cleanup) {
				depthMust(t, cleanup.Cleanup())
			}
			var refused memory.Failure
			if !errors.Is(failure, arena.ErrInjectedFailure) && (!errors.As(failure, &refused) || refused.Kind != memory.FailureMemout) {
				t.Fatal("unexpected selection failure", ordinal, failure)
			}
			if found || fact != nil {
				t.Fatal("failed selection published a fact", ordinal, failure)
			}
		} else {
			complete = true
			if !found || fact == nil || fact.state != p.reachFacts[0].state {
				t.Fatal("selection did not find interpreted case", ordinal)
			}
			depthMust(t, fact.Close())
		}
		if m.Stats().LiveReferences != before {
			t.Fatal("selection failure leaked references", ordinal, failure)
		}
		count, err := m.GetRefCount(h)
		depthMust(t, err)
		if count != caseCount {
			t.Fatal("selection changed case ownership", ordinal, count, caseCount)
		}
		currentMode, err := m.ProofMode()
		depthMust(t, err)
		if currentMode != mode {
			t.Fatal("selection changed proof mode")
		}
		fact, found, err = selectFact()
		depthMust(t, err)
		if !found || fact == nil || fact.state != p.reachFacts[0].state {
			t.Fatal("selection retry lost its case", ordinal)
		}
		depthMust(t, fact.Close())
		if shifted != (ast.ASTRef{}) {
			depthMust(t, m.DelRef(&shifted))
		}
		depthMust(t, md.Close())
	}
	if !complete || failures == 0 {
		t.Fatal("selection allocation campaign incomplete", complete, failures)
	}
}
