package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDirectFixedpointQueryCleanupFailureInvalidatesResults(t *testing.T) {
	for _, kind := range []string{"predicate", "predicates", "from-level"} {
		t.Run(kind, func(t *testing.T) {
			f := newSpacerGeneralizerExpressionFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			status, err := e.QueryExpression(f.query)
			must(err)
			if status != lbool.False {
				t.Fatal("expected unreachable expression query", status)
			}
			declaration, err := m.GetDecl(f.query)
			must(err)
			query := func() (lbool.LBool, error) {
				switch kind {
				case "predicate":
					return e.QueryPredicate(declaration)
				case "predicates":
					return e.QueryPredicates([]ast.ASTHandle{declaration})
				default:
					return e.QueryPredicateFromLevel(declaration, 0)
				}
			}
			prior := e.queryClone
			m.Budget().FailAfter(0)
			_, err = query()
			m.Budget().FailAfter(-1)
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal("expected previous-query cleanup refusal", err)
			}
			if e.queryClone != prior || prior.closed {
				t.Fatal("cleanup refusal discarded its live owner")
			}
			if _, err := e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("cleanup failure left the old result current", err)
			}
			if _, err := e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("cleanup failure exposed an old expression answer", err)
			}
			status, err = query()
			must(err)
			if status != lbool.True {
				t.Fatal("relation query did not recover after cleanup", status)
			}
			if !prior.closed || e.queryClone != nil {
				t.Fatal("retry did not finish expression-query disposal")
			}
		})
	}
}
