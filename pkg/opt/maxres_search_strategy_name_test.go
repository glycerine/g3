package opt

import "testing"

func TestMaxResSearchStrategyNamesMatchSourceTraceIDs(t *testing.T) {
	if maxResPrimal.name() != "maxres" || maxResPrimalDual.name() != "pd-maxres" {
		t.Fatal(maxResPrimal.name(), maxResPrimalDual.name())
	}
}
