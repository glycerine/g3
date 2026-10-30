package api

import (
	"math"
	"strings"
	"testing"

	lp "github.com/glycerine/g3/pkg/util/lp"
)

func TestSMTContextRunMPSOptimalModel(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	result, err := f.c.RunMPSString(apiOneVariableMPS("L", 5, ""), "optimal.mps", nil)
	astTestMust(t, err)
	if result.Status != lp.StatusOptimal || math.Abs(result.Objective-10) > 1e-9 || math.Abs(result.Columns["x"]-5) > 1e-9 {
		t.Fatal("MPS optimal result", result)
	}
	if !strings.Contains(result.Output, "status is OPTIMAL") || !strings.Contains(result.Output, "x = 5") {
		t.Fatal("MPS optimal output", result.Output)
	}
}
