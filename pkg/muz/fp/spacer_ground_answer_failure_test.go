package fp

// Source: spacer_context.cpp get_ground_sat_answer local witness construction.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestSpacerGroundAnswerAllocationFailurePublicationAndRetry(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 2048 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newExpressionFixture(t, "spacer", false)
			e, m := f.engine, f.manager
			fragmentMust(t, e.globals.SourceSetName("fixedpoint.xform.subsumption_checker", "false"))
			step, err := m.MkImplies(f.fact, f.applications["u"])
			fragmentMust(t, err)
			fragmentMust(t, e.addRuleEager(step, "numeric-ground-goal"))
			// The context's rule manager keeps the last Horn rule's head and
			// body (m_head, m_body); a first query settles them on the query
			// rule before caller ownership is sampled.
			_, err = e.QueryExpression(f.applications["u"])
			fragmentMust(t, err)
			fragmentMust(t, e.clearQueryResources())
			before, err := m.GetRefCount(f.fact)
			fragmentMust(t, err)
			status, err := e.QueryExpression(f.applications["u"])
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("query", status)
			}

			m.Budget().FailAfter(stop)
			answer, failure := e.ExpressionGroundSatAnswer()
			m.Budget().FailAfter(-1)
			if failure != nil {
				failures++
				if answer != (ast.ASTRef{}) {
					t.Fatal("failed conversion published a partial ground answer")
				}
				var cleanup memory.ConstructionFailure
				if errors.As(failure, &cleanup) {
					fragmentMust(t, cleanup.Cleanup())
				}
				var resource memory.Failure
				if !errors.Is(failure, arena.ErrInjectedFailure) && (!errors.As(failure, &resource) || resource.Kind != memory.FailureMemout) {
					t.Fatal("unexpected failure", failure)
				}
			} else {
				success = true
				fragmentMust(t, m.DelRef(&answer))
			}
			answer, err = e.ExpressionGroundSatAnswer()
			fragmentMust(t, err)
			witness, err := m.RefGet(answer)
			fragmentMust(t, err)
			want, err := m.MkAndArray([]ast.ASTHandle{f.applications["u"], f.fact})
			fragmentMust(t, err)
			if witness != want {
				t.Fatal("ground witness omitted original goal or numeric premise", witness, want)
			}
			fragmentMust(t, m.DelRef(&answer))
			// The factory owns local solver/proof references until the query
			// context is disposed; compare caller ownership only after disposal.
			fragmentMust(t, e.clearQueryResources())
			if got, err := m.GetRefCount(f.fact); err != nil || got != before {
				t.Fatal("query disposal lost caller fact ownership", before, got, err)
			}
			m.Budget().FailAfter(0)
			retired, retiredErr := e.ExpressionGroundSatAnswer()
			m.Budget().FailAfter(-1)
			if retired != (ast.ASTRef{}) || !errors.Is(retiredErr, ErrPropositionalEngine) {
				t.Fatal("retired query reached ground-answer construction", retired, retiredErr)
			}
		})
		if t.Failed() {
			break
		}
	}
	if failures == 0 || !success {
		t.Fatal("incomplete ground-answer construction frontier", failures, success)
	}
}
