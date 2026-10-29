package shell_test

import (
	"strings"
	"testing"
)

func TestLPFrontendInfeasibleAndUnboundedStatuses(t *testing.T) {
	for _, tc := range []struct {
		name, input, status string
	}{
		{"infeasible", oneVariableMPS("G", 5, "BOUNDS\n"+mpsBound("UP", "x 2")), "INFEASIBLE"},
		{"unbounded", oneVariableMPS("G", 0, ""), "UNBOUNDED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, out := lpFrontendFixture(t, tc.input, nil)
			if err := f.Run(); err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(out.String(), "status is "+tc.status+"\n") || strings.Contains(out.String(), "column values") {
				t.Fatalf("status output %q", out.String())
			}
		})
	}
}
