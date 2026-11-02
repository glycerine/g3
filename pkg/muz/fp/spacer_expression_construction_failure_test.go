package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestSpacerExpressionConstructionFailureWithoutPrequeryOwner(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop <= 32768 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newExpressionFixture(t, "spacer", false)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			// Exercise construction and disposal of a new query context; the other
			// allocation campaign verifies transfer of a caller's pre-query context.
			// gparams::check_registered commits its one-shot flag before
			// registration. Its allocation-failure prefix is deliberately
			// non-retryable and covered by util/params source fault tests.
			// Initialize that process metadata outside recoverable query limits.
			_, err := e.globals.SourceModuleDescriptorsName("smt")
			must(err)
			if e.spacer != nil || e.queryClone != nil {
				t.Fatal("construction fixture warmed an engine context")
			}
			budget := m.Budget()
			budget.FailAfter(stop)
			status, err := e.QueryExpression(f.query)
			budget.FailAfter(-1)
			if err != nil {
				failures++
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					must(cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected query construction failure", err)
				}
				if _, err := e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("failed construction published a result", err)
				}
				if _, err := e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("failed construction published an answer", err)
				}
				status, err = e.QueryExpression(f.query)
				must(err)
			} else {
				success = true
			}
			if status != lbool.True {
				t.Fatal("query/retry did not reach zero", stop, status)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !success || failures == 0 {
		t.Fatal("missing construction failure/success frontier", failures, success)
	}
}
