package sat

import (
	"strings"
	"testing"
)

func TestParseDIMACSRetainsClauseOrderAndSigns(t *testing.T) {
	input := "c example\np cnf 4 3\n1 -2 0\n4 0\n0\n"
	cnf, err := ReadDIMACSCNF(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if cnf.VariableCount != 4 || len(cnf.Clauses) != 3 {
		t.Fatal(cnf.VariableCount, cnf.Clauses)
	}
	if len(cnf.Clauses[0]) != 2 || cnf.Clauses[0][0] != 1 || cnf.Clauses[0][1] != -2 || len(cnf.Clauses[2]) != 0 {
		t.Fatal(cnf.Clauses)
	}
}
