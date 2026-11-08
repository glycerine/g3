package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestPDRExpressionPrequeryOwnerFailureAndRetry(t *testing.T) {
	failures, preparationFailures, solveFailures, success := 0, 0, 0, false
	for stop := int64(0); stop <= 32768 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newCoveredPDRExpressionFixture(t)
			e, m := f.engine, f.manager
			budget := m.Budget()
			defer budget.FailAfter(-1)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			declaration, property, query, fact := f.declaration, f.property, f.query, f.fact
			owner := e.pdr
			budget.FailAfter(stop)
			status, err := e.QueryExpression(query)
			budget.FailAfter(-1)
			if err != nil {
				failures++
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					must(cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected query failure", err)
				}
				if _, err := e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("failed query published a result", err)
				}
				if _, err := e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("failed query published an answer", err)
				}
				if e.queryClone != nil {
					solveFailures++
					if e.queryClone.pdr != owner {
						t.Fatal("failed solve lost its original context owner")
					}
					initialized, err := owner.Initialized()
					must(err)
					if !initialized {
						t.Fatal("failed-solve fixture did not install rules")
					}
				} else {
					preparationFailures++
					if e.pdr != owner {
						t.Fatal("failed preparation replaced its original owner")
					}
					initialized, err := owner.Initialized()
					must(err)
					if initialized {
						t.Fatal("failed preparation published a program")
					}
				}
				cover, err := e.CoverDelta(2, declaration)
				must(err)
				defer func() { must(m.DelRef(&cover)) }()
				handle, err := m.RefGet(cover)
				must(err)
				if handle != property {
					t.Fatal("failed query lost the pre-query cover", handle, property)
				}
				if e.queryClone != nil {
					must(e.addRuleEager(fact, "duplicate-after-failed-query"))
					if _, err := e.CoverDelta(2, declaration); !errors.Is(err, ErrPropositionalEngine) {
						t.Fatal("rule mutation exposed stale failed-query frames", err)
					}
				}
				status, err = e.QueryExpression(query)
				must(err)
			} else {
				success = true
			}
			if status != lbool.True {
				t.Fatal("query/retry did not reach zero", status)
			}
		})
		if t.Failed() {
			break
		}
	}
	if failures == 0 || preparationFailures == 0 || solveFailures == 0 || !success {
		t.Fatal("missing preparation/solve failure and success frontiers", failures, preparationFailures, solveFailures, success)
	}
}
