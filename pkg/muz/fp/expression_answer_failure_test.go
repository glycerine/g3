package fp

// Source: PDR/Spacer get_answer and model-converter replay into predicate definitions.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestExpressionAnswerAllocationFailurePublicationAndRetry(t *testing.T) {
	for _, backend := range []string{"pdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			success, failures := false, 0
			for stop := int64(0); stop < 2048 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					f := newExpressionFixture(t, backend, false)
					e, m := f.engine, f.manager
					one, err := m.ArithmeticIntegerNumeralText("1")
					fragmentMust(t, err)
					query, err := m.MkApp(f.declaration, []ast.ASTHandle{one})
					fragmentMust(t, err)
					qr, err := m.NewRef(query)
					fragmentMust(t, err)
					defer func() { fragmentMust(t, m.DelRef(&qr)) }()
					status, err := e.QueryExpression(query)
					fragmentMust(t, err)
					if status != lbool.False {
						t.Fatal("query", status)
					}
					conversions := e.queryClone.pdrConversions
					if backend == "spacer" {
						conversions = e.queryClone.spacerConversions
					}
					cone := false
					for _, conversion := range conversions {
						cone = cone || conversion.kind == hornConvertCone
					}
					if !cone {
						t.Fatal("fixture did not exercise model conversion")
					}
					before := m.Stats().LiveReferences
					m.Budget().FailAfter(stop)
					answerRef, failure := e.ExpressionAnswer()
					m.Budget().FailAfter(-1)
					if failure != nil {
						failures++
						if answerRef != (ast.ASTRef{}) {
							t.Fatal("failed conversion published a partial answer")
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
						fragmentMust(t, m.DelRef(&answerRef))
					}
					// m_last_answer keeps the published answer until the next
					// query; release it so the accounting sees only the caller's.
					fragmentMust(t, e.releaseLastAnswer())
					if got := m.Stats().LiveReferences; got != before {
						t.Fatal("failed publication leaked references", stop, before, got, failure)
					}
					answerRef, err = e.ExpressionAnswer()
					fragmentMust(t, err)
					answer, err := m.RefGet(answerRef)
					fragmentMust(t, err)
					solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
					fragmentMust(t, err)
					fragmentMust(t, solver.Assert(answer))
					status, err = solver.CheckSat(nil)
					fragmentMust(t, err)
					if status != lbool.True {
						t.Fatal("retry answer inconsistent", status)
					}
					for _, item := range []struct {
						formula ast.ASTHandle
						truth   bool
					}{{f.fact, true}, {query, false}, {f.applications["p"], true}, {f.applications["q"], true}} {
						obligation := item.formula
						if item.truth {
							obligation, err = m.MkNot(obligation)
							fragmentMust(t, err)
						}
						fragmentMust(t, solver.Push())
						fragmentMust(t, solver.Assert(obligation))
						status, err = solver.CheckSat(nil)
						fragmentMust(t, err)
						if status != lbool.False {
							t.Fatal("answer does not imply original fact or refute query", item.truth, status)
						}
						fragmentMust(t, solver.Pop(1))
					}
					fragmentMust(t, solver.Close())
					fragmentMust(t, m.DelRef(&answerRef))
					fragmentMust(t, e.releaseLastAnswer())
					if got := m.Stats().LiveReferences; got != before {
						t.Fatal("retry leaked references", before, got)
					}
					held, err := e.ExpressionAnswer()
					fragmentMust(t, err)
					heldHandle, err := m.RefGet(held)
					fragmentMust(t, err)
					fragmentMust(t, e.addRuleEager(f.fact, "retire-answer-query"))
					m.Budget().FailAfter(0)
					retired, retiredErr := e.ExpressionAnswer()
					m.Budget().FailAfter(-1)
					if retired != (ast.ASTRef{}) || !errors.Is(retiredErr, ErrPropositionalEngine) {
						t.Fatal("retired query reached answer construction", retired, retiredErr)
					}
					fragmentMust(t, e.clearQueryResources())
					afterClose, err := m.RefGet(held)
					fragmentMust(t, err)
					if afterClose != heldHandle {
						t.Fatal("caller answer changed after query disposal")
					}
					fragmentMust(t, m.DelRef(&held))
				})
				if t.Failed() {
					break
				}
			}
			if failures == 0 || !success {
				t.Fatal("incomplete answer construction frontier", failures, success)
			}
		})
	}
}
