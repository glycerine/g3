package opt

// Source: src/opt/maxres.cpp strategy selection and trace identity.

import "testing"

func TestMaxResStrategySelectsOnlyExplicitPrimalDualEngine(t *testing.T) {
	for _, row := range []struct {
		engine string
		want   maxResStrategy
		name   string
	}{{"", maxResPrimal, "maxres"}, {"maxres", maxResPrimal, "maxres"}, {"unknown", maxResPrimal, "maxres"}, {"pd-maxres", maxResPrimalDual, "pd-maxres"}} {
		got := maxResStrategyFromConfig(maxSMTConfig{engine: row.engine})
		if got != row.want || got.name() != row.name {
			t.Fatal("MaxRes strategy", row.engine, got, got.name())
		}
	}
}
