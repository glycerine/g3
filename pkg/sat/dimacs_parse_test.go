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

// AUDIT.md sat_01-F6: the source scanner ends a number at the first
// non-digit and needs no whitespace before a sign.
func TestReadDIMACSCNFUsesSourceScanner(t *testing.T) {
	cnf, err := ReadDIMACSCNF(strings.NewReader("p cnf 3 2\n1-2 0\n2 +3 0 c tail\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cnf.VariableCount != 3 || len(cnf.Clauses) != 2 || len(cnf.Clauses[0]) != 2 || len(cnf.Clauses[1]) != 2 ||
		cnf.Clauses[0][0] != 1 || cnf.Clauses[0][1] != -2 || cnf.Clauses[1][0] != 2 || cnf.Clauses[1][1] != 3 {
		t.Fatal(cnf.VariableCount, cnf.Clauses)
	}
}
