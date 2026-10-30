package api

import (
	"testing"

	lp "github.com/glycerine/g3/pkg/util/lp"
)

func TestSMTContextRunMPSNonoptimalStatuses(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	for _, test := range []struct {
		name, input string
		status      lp.Status
	}{
		{"infeasible", apiOneVariableMPS("G", 5, "BOUNDS\n"+apiMPSBound("UP", "x 2")), lp.StatusInfeasible},
		{"unbounded", apiOneVariableMPS("G", 0, ""), lp.StatusUnbounded},
	} {
		result, err := f.c.RunMPSString(test.input, test.name+".mps", nil)
		astTestMust(t, err)
		if result.Status != test.status || result.Columns != nil {
			t.Fatal(test.name, result)
		}
	}
}
