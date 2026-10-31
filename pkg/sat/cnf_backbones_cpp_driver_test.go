package sat

// Source: src/test/cnf_backbones.cpp, DIMACS and get_consequences mode.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestCNFBackbonesSourceConsequences(t *testing.T) {
	const input = "p cnf 4 4\n1 0\n-1 2 0\n-2 3 0\n4 -4 0\n"
	solver, _, _ := satSolverCheckFixture(t)
	satOK(t, ParseDIMACS(strings.NewReader(input), solver))
	status, rows, err := solver.GetConsequences(nil, []BoolVar{1, 2, 3, 4})
	satOK(t, err)
	if status != lbool.True {
		t.Fatal("consequence status", status)
	}
	seen := make(map[BoolVar]bool)
	for _, row := range rows {
		if len(row) != 1 || row[0].sign() || row[0].variable() < 1 || row[0].variable() > 3 {
			t.Fatal("unexpected backbone", row)
		}
		seen[row[0].variable()] = true
	}
	if len(seen) != 3 {
		t.Fatal("missing backbone", seen, rows)
	}
}
