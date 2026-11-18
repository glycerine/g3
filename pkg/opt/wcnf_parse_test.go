package opt

import (
	"errors"
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

// AUDIT.md opt_02-F5: wcnf::parse accepts what the old line-based parser
// rejected; only parse_int's unexpected character is an error.
func TestParseWCNFRejectsOnlyMalformedLiterals(t *testing.T) {
	for _, input := range []string{"p wcnf 1 1 2\n2 1\n", "p wcnf 1 1 2\n2 a 0\n", "1 1 - 0\n"} {
		_, err := ParseWCNF(strings.NewReader(input))
		var exit *InputParseExit
		if !errors.As(err, &exit) || exit.Status != 3 {
			t.Fatalf("accepted %q: %v", input, err)
		}
	}
	for _, input := range []string{"1 1 0\n", "p cnf 1 1 2\n2 1 0\n", "p wcnf 1 1 2\n2 2 0\n", "p wcnf 1 2 2\n2 1 0\n"} {
		if _, err := ParseWCNF(strings.NewReader(input)); err != nil {
			t.Fatalf("rejected %q: %v", input, err)
		}
	}
}

func TestParseWCNFMultiLineClause(t *testing.T) {
	d, err := ParseWCNF(strings.NewReader("p wcnf 2 3 10\n1 1\n 0\n1 2 0\n10 -1 -2 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []WCNFClause{
		{1, []WCNFLiteral{{1, false}}},
		{1, []WCNFLiteral{{2, false}}},
		{10, []WCNFLiteral{{1, true}, {2, true}}},
	}
	if len(d.Clauses) != len(want) {
		t.Fatal(d.Clauses)
	}
	for i := range want {
		got := d.Clauses[i]
		if got.Weight != want[i].Weight || len(got.Literals) != len(want[i].Literals) {
			t.Fatal(i, got)
		}
		for j := range got.Literals {
			if got.Literals[j] != want[i].Literals[j] {
				t.Fatal(i, got)
			}
		}
	}
}

func TestParseWCNFMissingTop(t *testing.T) {
	d, err := ParseWCNF(strings.NewReader("p wcnf 2 3\n1 1 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d.TopWeight != uint64(^uint32(0)) || d.NumVariables != 2 || d.NumClauses != 3 {
		t.Fatal(d.TopWeight, d.NumVariables, d.NumClauses)
	}
	// parse_unsigned wraps at 32 bits.
	d, err = ParseWCNF(strings.NewReader("p wcnf 1 1 4294967297\n4294967298 1 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d.TopWeight != 1 || d.Clauses[0].Weight != 2 {
		t.Fatal(d.TopWeight, d.Clauses[0].Weight)
	}
}

// FORK FIX #134: parse_int accumulates in unsigned, so a literal past
// INT_MAX wraps modulo 2^32 with defined behavior, and the magnitude of
// INT_MIN is 2147483648 (tst_z3_4_7_1_opt_parse_int_wrap).
func TestParseWCNFLiteralWrap(t *testing.T) {
	d, err := ParseWCNF(strings.NewReader("p wcnf 3 2 9\n9 4294967298 -4294967298 0\n9 -2147483648 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := [][]WCNFLiteral{{{Variable: 2}, {Variable: 2, Negative: true}}, {{Variable: 2147483648, Negative: true}}}
	if len(d.Clauses) != len(want) {
		t.Fatal(d.Clauses)
	}
	for i, c := range d.Clauses {
		if len(c.Literals) != len(want[i]) {
			t.Fatal(i, c.Literals)
		}
		for j, l := range c.Literals {
			if l != want[i][j] {
				t.Fatal(i, j, l)
			}
		}
	}
}
