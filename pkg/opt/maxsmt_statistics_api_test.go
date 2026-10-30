package opt

// Source: src/opt/maxsmt.h defensive statistics publication.

import "testing"

func TestMaxSMTResultStatisticsReturnsDefensiveCopy(t *testing.T) {
	solver := &ArithmeticSolver{}
	result := &ArithmeticResult{MaxSMT: []MaxSMTStatistics{{Engine: "maxres", Checks: 3}, {Engine: "sortmax", Inputs: 4}}}
	statistics, err := solver.MaxSMTResultStatistics(result)
	if err != nil || len(statistics) != 2 {
		t.Fatal(statistics, err)
	}
	statistics[0].Checks = 99
	again, err := solver.MaxSMTResultStatistics(result)
	if err != nil || again[0].Checks != 3 {
		t.Fatal("statistics alias", again, err)
	}
}
