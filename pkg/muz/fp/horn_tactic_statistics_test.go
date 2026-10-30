package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
)

func TestHornTacticStatisticsAccumulateAndReset(t *testing.T) {
	f := newHornLifecycleFixture(t, false)
	f.populate(t)
	result := &tactic.TacticResult{}
	t.Cleanup(func() { fpMust(t, result.Close()) })
	fpMust(t, f.tactic.Apply(f.goal, result))
	if got := f.tactic.CollectStatistics()["applications"]; got != 1 {
		t.Fatal("application statistics", got)
	}
	fpMust(t, f.tactic.ResetStatistics())
	if len(f.tactic.CollectStatistics()) != 0 {
		t.Fatal("statistics survived reset")
	}
}
