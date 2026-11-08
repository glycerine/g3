package fp

// Source: dl_bmc_engine.cpp bmc::query resets the solver before preparing a
// new expression query; collect_statistics reports that solver even on failure.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func bmcExpressionCheckCount(t *testing.T, engine *PropositionalEngine) uint32 {
	t.Helper()
	stats := statistics.New()
	fragmentMust(t, engine.CollectStatistics(stats))
	var count uint32
	for i := 0; i < stats.Size(); i++ {
		if stats.IsUint(i) && stats.Key(i) == "num checks" {
			count += stats.UintValue(i)
		}
	}
	return count
}

func TestBMCExpressionStatisticsReplacePreviousQueryAfterCancellation(t *testing.T) {
	for _, backend := range []string{"bmc", "qbmc"} {
		t.Run(backend, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			fragmentMust(t, e.SelectEngine(backend))
			query := f.applications["p"]
			fragmentMust(t, e.addRuleEager(query, "statistics-fact"))
			status, err := e.QueryExpression(query)
			fragmentMust(t, err)
			if status != lbool.True || bmcExpressionCheckCount(t, e) == 0 {
				t.Fatal("initial query did not perform a solver check", status)
			}
			m.ResourceLimit().IncCancel()
			status, err = e.QueryExpression(query)
			m.ResourceLimit().DecCancel()
			if status != lbool.Undef || err == nil || !strings.Contains(err.Error(), messages.Canceled) {
				t.Fatal("query did not observe cancellation", status, err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				if got := bmcExpressionCheckCount(t, e); got != 0 {
					t.Fatal("canceled query exposed prior solver checks", got)
				}
			}
			if _, err = e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("statistics transfer published a failed result", err)
			}
			if _, err = e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("statistics transfer published a failed answer", err)
			}
			reason, err := e.ReasonUnknown()
			fragmentMust(t, err)
			if reason != "canceled" {
				t.Fatal("cancellation reason", reason)
			}
			status, err = e.QueryExpression(query)
			fragmentMust(t, err)
			if status != lbool.True || bmcExpressionCheckCount(t, e) == 0 {
				t.Fatal("query retry did not publish new solver statistics", status)
			}
			answer, err := e.ExpressionAnswer()
			fragmentMust(t, err)
			fragmentMust(t, m.DelRef(&answer))
			fragmentMust(t, e.ResetStatistics())
			if got := bmcExpressionCheckCount(t, e); got != 0 {
				t.Fatal("statistics reset", got)
			}
		})
	}
}
