package shell_test

import "testing"

func TestLPFrontendMinimumFlipsAndRestoresObjective(t *testing.T) {
	bounds := "BOUNDS\n" + mpsBound("LO", "x 2") + mpsBound("UP", "x 5")
	f, out := lpFrontendFixture(t, oneVariableMPS("L", 5, bounds), map[string]string{"lp.min": "true", "lp.rep_freq": "7"})
	if err := f.Run(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "extended A[1,2]\nstatus is OPTIMAL\nobjective = 4\ncolumn values\nx = 2\n" {
		t.Fatalf("minimum source objective %q", out.String())
	}
}
