package shell_test

import (
	"strings"
	"testing"
)

// AUDIT.md util_lp_04-F1: the MPS reader converts numbers with atof, so
// trailing text after a numeric field is ignored and the LP is solved.
func TestLPFrontendAtofTrailingText(t *testing.T) {
	input := oneVariableMPS("L", 4, "")
	rhs := mpsPair("RHS1", "LIMIT", 4)
	if !strings.Contains(input, rhs) {
		t.Fatal("fixture RHS line", input)
	}
	input = strings.Replace(input, rhs, strings.TrimSuffix(rhs, "\n")+"x\n", 1)
	f, out := lpFrontendFixture(t, input, nil)
	if err := f.Run(); err != nil {
		t.Fatal(err, out.String())
	}
	want := "extended A[1,2]\nstatus is OPTIMAL\nobjective = 8\ncolumn values\nx = 4\n"
	if out.String() != want {
		t.Fatalf("LP frontend output %q", out.String())
	}
}
