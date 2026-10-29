package shell_test

import (
	"strings"
	"testing"
)

func TestLPFrontendMaximalOptimalModel(t *testing.T) {
	f, out := lpFrontendFixture(t, oneVariableMPS("L", 5, ""), nil)
	if err := f.Run(); err != nil {
		t.Fatal(err)
	}
	want := "extended A[1,2]\nstatus is OPTIMAL\nobjective = 10\ncolumn values\nx = 5\n"
	if out.String() != want || !strings.Contains(out.String(), "x = 5") {
		t.Fatalf("LP frontend output %q", out.String())
	}
}
