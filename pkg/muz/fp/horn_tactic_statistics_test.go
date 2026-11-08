package fp_test

import (
	"github.com/glycerine/g3/pkg/util/statistics"
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
)

func TestHornTacticStatisticsAccumulateAndReset(t *testing.T) {
	f := newHornLifecycleFixture(t, false)
	f.populate(t)
	result := &tactic.TacticResult{}
	t.Cleanup(func() { fpMust(t, result.Close()) })
	fpMust(t, f.tactic.Apply(f.goal, result))
	// horn_tactic keeps only what the engine reports (m_stats); the
	// propositional engine reports no nonzero counters here.
	_ = statistics.Sums(f.tactic.CollectStatistics)
	fpMust(t, f.tactic.ResetStatistics())
	if len(statistics.Sums(f.tactic.CollectStatistics)) != 0 {
		t.Fatal("statistics survived reset")
	}
}
