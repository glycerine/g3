package fp

// Source: PDR/Spacer dl_interface::get_model and model-converter replay.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestExpressionModelAllocationFailurePublicationAndRetry(t *testing.T) {
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
					md, failure := e.ExpressionModel()
					m.Budget().FailAfter(-1)
					if failure != nil {
						failures++
						if md != nil {
							t.Fatal("failed conversion published a partial model")
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
						fragmentMust(t, md.Close())
					}
					if got := m.Stats().LiveReferences; got != before {
						t.Fatal("failed publication leaked references", stop, before, got, failure)
					}
					md, err = e.ExpressionModel()
					fragmentMust(t, err)
					cooperation, err := cooperate.NewCooperation(m.Budget())
					fragmentMust(t, err)
					evaluator, err := model.NewModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: e.params, Globals: e.globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: cooperation})
					fragmentMust(t, err)
					fragmentMust(t, evaluator.SetModelCompletion(false))
					for _, item := range []struct {
						formula ast.ASTHandle
						truth   bool
					}{{f.fact, true}, {query, false}, {f.applications["p"], true}, {f.applications["q"], true}} {
						value, err := evaluator.EvaluateValue(item.formula)
						fragmentMust(t, err)
						h, err := m.RefGet(value)
						fragmentMust(t, err)
						if (item.truth && !m.IsTrue(h)) || (!item.truth && !m.IsFalse(h)) {
							t.Fatal("retry lost original-model semantics", item.truth, h)
						}
						fragmentMust(t, m.DelRef(&value))
					}
					fragmentMust(t, evaluator.Close())
					fragmentMust(t, cooperation.Close())
					fragmentMust(t, md.Close())
					if got := m.Stats().LiveReferences; got != before {
						t.Fatal("retry leaked references", before, got)
					}
					fragmentMust(t, e.addRuleEager(f.fact, "retire-model-query"))
					m.Budget().FailAfter(0)
					retired, retiredErr := e.ExpressionModel()
					m.Budget().FailAfter(-1)
					if retired != nil || !errors.Is(retiredErr, ErrPropositionalEngine) {
						t.Fatal("retired query reached model construction", retired, retiredErr)
					}
				})
				if t.Failed() {
					break
				}
			}
			if failures == 0 || !success {
				t.Fatal("incomplete model construction frontier", failures, success)
			}
		})
	}
}
