package portfolio_test

import (
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	"testing"
)

func TestFDTacticStatisticsLifecycle(t *testing.T) {
	f := enumSolverFixture(t)
	r, e := portfolio.NewFDTactic(f.manager, nil, f.services)
	enumMust(t, e)
	enumMust(t, r.IncRef())
	defer r.DecRef()
	if r.CollectStatistics() == nil {
		t.Fatal("nil stats")
	}
	enumMust(t, r.ResetStatistics())
	enumMust(t, r.Cleanup())
}
