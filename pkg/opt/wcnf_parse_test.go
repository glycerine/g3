package opt

import (
	"strings"
	"testing"
)

func TestParseWCNFPreservesHeaderWeightsAndLiteralOrder(t *testing.T) {
	d, err := ParseWCNF(strings.NewReader("c sample\np wcnf 3 2 10\n10 1 -2 0\n3 -3 2 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d.NumVariables != 3 || d.NumClauses != 2 || d.TopWeight != 10 || len(d.Clauses) != 2 {
		t.Fatal(d)
	}
	if d.Clauses[1].Weight != 3 || len(d.Clauses[1].Literals) != 2 || !d.Clauses[1].Literals[0].Negative || d.Clauses[1].Literals[0].Variable != 3 {
		t.Fatal(d.Clauses[1])
	}
	copy := d.Copy()
	copy.Clauses[0].Literals[0].Variable = 99
	if d.Clauses[0].Literals[0].Variable != 1 {
		t.Fatal("copy aliases source")
	}
}

func TestParseWCNFRejectsMalformedAndOutOfRangeInput(t *testing.T) {
	for _, input := range []string{
		"1 1 0\n", "p cnf 1 1 2\n2 1 0\n", "p wcnf 1 1 2\n2 2 0\n",
		"p wcnf 1 1 2\n2 1\n", "p wcnf 1 2 2\n2 1 0\n",
	} {
		if _, err := ParseWCNF(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}
