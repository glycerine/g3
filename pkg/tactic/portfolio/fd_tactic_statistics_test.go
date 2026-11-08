package portfolio_test

import (
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	"github.com/glycerine/g3/pkg/util/statistics"
	"testing"
)

func TestFDTacticStatisticsLifecycle(t *testing.T) {
	f := enumSolverFixture(t)
	r, e := portfolio.NewFDTactic(f.manager, nil, f.services)
	enumMust(t, e)
	enumMust(t, r.IncRef())
	defer r.DecRef()
	// Before any apply there is nothing to report (statistics::update skips
	// zero counters).
	if got := statistics.Sums(r.CollectStatistics); len(got) != 0 {
		t.Fatal("statistics before apply", got)
	}
	enumMust(t, r.ResetStatistics())
	enumMust(t, r.Cleanup())
}
