package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestQueryCloneCleanupOwnsRefusedDisposalAndStatistics(t *testing.T) {
	for _, completion := range []string{"clear", "statistics-reset", "parameter-update", "query-retry", "close"} {
		t.Run(completion, func(t *testing.T) {
			f := newSpacerGeneralizerExpressionFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			count := func(engine *PropositionalEngine, key statistics.Key) uint32 {
				t.Helper()
				s := statistics.New()
				must(engine.CollectStatistics(s))
				var n uint32
				for i := 0; i < s.Size(); i++ {
					if s.IsUint(i) && s.Key(i) == key {
						n += s.UintValue(i)
					}
				}
				return n
			}
			status, err := e.QueryExpression(f.query)
			must(err)
			if status != lbool.False {
				t.Fatal("expected unreachable arithmetic program", status)
			}
			checks, generalizers := count(e, "num checks"), count(e, "bool inductive gen")
			if checks == 0 || generalizers == 0 {
				t.Fatal("fixture produced no source counters")
			}
			clone := e.queryClone
			// Discard an actual solver-owning clone through the same cleanup entry
			// used by failed temporary construction/preprocessing, without publishing it.
			e.queryClone = nil
			e.queryCloneRevision = 0
			e.queryClonePredicate = ast.ASTHandle{}
			e.queryReady = false
			m.Budget().FailAfter(0)
			err = e.discardQueryClone(clone)
			m.Budget().FailAfter(-1)
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal("expected an actual refused guard-disabling allocation", err)
			}
			if len(e.queryCleanup) != 1 || e.queryCleanup[0].clone != clone || clone.closed || e.queryClone != nil {
				t.Fatal("refused temporary disposal lost or published its owner")
			}
			pending := e.queryCleanup[0]
			if !pending.statisticsReady {
				t.Fatal("disposal did not preserve its pre-close statistics")
			}
			if _, err := e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("cleanup owner was exposed as a query result", err)
			}
			if _, err := e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("cleanup owner was exposed as an answer", err)
			}
			if got := count(e, "num checks"); got != checks {
				t.Fatal("pending disposal lost or duplicated check statistics", got, checks)
			}
			if got := count(e, "bool inductive gen"); got != generalizers {
				t.Fatal("pending disposal lost generalizer statistics", got, generalizers)
			}
			m.Budget().FailAfter(0)
			err = e.clearQueryResources()
			m.Budget().FailAfter(-1)
			if !errors.Is(err, arena.ErrInjectedFailure) || len(e.queryCleanup) != 1 || e.queryCleanup[0] != pending {
				t.Fatal("repeated cleanup refusal changed its owner", err)
			}
			switch completion {
			case "clear":
				must(e.clearQueryResources())
			case "statistics-reset":
				must(e.ResetStatistics())
				if count(e, "bool inductive gen") != 0 {
					t.Fatal("statistics reset left pending generalizer history")
				}
			case "parameter-update":
				must(e.SetQueryParameters(nil))
				checks = 0
			case "query-retry":
				status, err = e.QueryExpression(f.query)
				must(err)
				if status != lbool.False {
					t.Fatal("query retry result", status)
				}
				checks = count(e, "num checks")
			case "close":
				must(e.Close())
				must(e.Close())
				if len(e.queryCleanup) != 0 || !clone.closed {
					t.Fatal("engine close lost pending query ownership")
				}
				return
			}
			if len(e.queryCleanup) != 0 || !clone.closed {
				t.Fatal("cleanup did not finish after the limit was removed")
			}
			if got := count(e, "num checks"); got != checks {
				t.Fatal("cleanup completion changed archived checks", got, checks)
			}
			must(e.clearQueryResources())
			if got := count(e, "num checks"); got != checks {
				t.Fatal("repeated cleanup duplicated archived checks", got, checks)
			}
			status, err = e.QueryExpression(f.query)
			must(err)
			if status != lbool.False {
				t.Fatal("query did not recover after disposal", status)
			}
			if got := count(e, "num checks"); got != checks+count(e.queryClone, "num checks") {
				t.Fatal("retry lost the completed query history", got)
			}
		})
	}
}
