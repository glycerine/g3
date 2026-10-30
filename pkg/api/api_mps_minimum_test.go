package api

import (
	"math"
	"testing"

	lp "github.com/glycerine/g3/pkg/util/lp"
)

func TestSMTContextRunMPSMinimum(t *testing.T) {
	bounds := "BOUNDS\n" + apiMPSBound("LO", "x 2") + apiMPSBound("UP", "x 5")
	f := newSMTAPIFixture(t, "ALL", nil)
	result, err := f.c.RunMPSString(apiOneVariableMPS("L", 5, bounds), "minimum.mps", &MPSRunOptions{Minimum: true})
	astTestMust(t, err)
	if result.Status != lp.StatusOptimal || math.Abs(result.Objective-4) > 1e-9 || math.Abs(result.Columns["x"]-2) > 1e-9 {
		t.Fatal("MPS minimum result", result)
	}
}
