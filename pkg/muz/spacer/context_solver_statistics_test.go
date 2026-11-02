package spacer

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestSpacerContextCountsSharedSolverStatisticsOnce(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	c := f.context
	status, err := c.Solve(0, 4)
	depthMust(t, err)
	if status != lbool.True {
		t.Fatal("expected reachable fixture", status)
	}
	ownerStats := statistics.New()
	depthMust(t, c.owner.CollectStatistics(ownerStats))
	sum := func(s *statistics.Statistics, key statistics.Key) uint32 {
		var result uint32
		for i := 0; i < s.Size(); i++ {
			if s.IsUint(i) && s.Key(i) == key {
				result += s.UintValue(i)
			}
		}
		return result
	}
	expected := sum(ownerStats, "num checks")
	if expected == 0 {
		t.Fatal("fixture performed no solver checks")
	}
	for round := 0; round < 2; round++ {
		collected := statistics.New()
		depthMust(t, c.CollectStatistics(collected))
		if actual := sum(collected, "num checks"); actual != expected {
			t.Fatal("shared backend counters were duplicated through predicate views", actual, expected)
		}
	}
}
