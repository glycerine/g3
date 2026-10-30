package opt

// Source: src/opt/maxres.cpp get_cores stopping order.

import "testing"

func TestMaxResCoreBatchLimitsApplyPerRound(t *testing.T) {
	for _, row := range []struct {
		config      maxSMTConfig
		count, size int
		want        bool
	}{{maxSMTConfig{}, 0, 100, false}, {maxSMTConfig{maxNumCores: 0}, 1, 1, true}, {maxSMTConfig{maxNumCores: 2}, 1, 1, false}, {maxSMTConfig{maxNumCores: 2}, 2, 1, true}, {maxSMTConfig{maxNumCores: 9, maxCoreSize: 3}, 1, 2, false}, {maxSMTConfig{maxNumCores: 9, maxCoreSize: 3}, 1, 3, true}} {
		engine := &maxResEngine{config: row.config}
		if got := engine.coreBatchLimitReached(row.count, row.size); got != row.want {
			t.Fatal("core batch limit", row, got)
		}
	}
}
