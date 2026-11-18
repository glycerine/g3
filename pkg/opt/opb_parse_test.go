package opt

import (
	"strings"
	"testing"
)

func TestParseOPBOptionalConstraintSemicolonsAndObjectiveEOF(t *testing.T) {
	for _, input := range []string{
		"1 x1 >= 1\n1 ~x1 >= 1\nmin: 2 x1",
		"1 x1 >= 1; 1 ~x1 >= 1; min: 2 x1   \n",
	} {
		document, err := ParseOPB(strings.NewReader(input))
		if err != nil {
			t.Fatalf("ParseOPB(%q): %v", input, err)
		}
		s := document.Statements
		if len(s) != 3 || s[0].Objective || s[1].Objective || !s[2].Objective ||
			len(s[2].Terms) != 1 || s[2].Terms[0].Coefficient != "2" {
			t.Fatalf("ParseOPB(%q): %#v", input, document)
		}
	}
}

func TestParseOPBObjectivesConstraintsAndProducts(t *testing.T) {
	input := `* #variable= 3 #constraint= 2
min: + 2 x1 -3 ~x2 x3;
+1 x1 +4 ~x2 >= 1;
-2 x1 x3 <= + 0;
`
	document, err := ParseOPB(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	s := document.Statements
	if len(s) != 3 || !s[0].Objective || !s[0].MinimizeSyntax || s[1].Objective || s[2].Objective {
		t.Fatalf("document = %#v", document)
	}
	terms := s[0].Terms
	if len(terms) != 2 || terms[0].Coefficient != "2" || terms[1].Coefficient != "-3" {
		t.Fatalf("objective terms = %#v", terms)
	}
	if len(terms[1].Literals) != 2 || terms[1].Literals[0] != (OPBLiteral{Variable: 2, Negative: true}) || terms[1].Literals[1] != (OPBLiteral{Variable: 3}) {
		t.Fatalf("product = %#v", terms[1].Literals)
	}
	if s[1].Relation != OPBGreaterEqual || s[1].Bound != "1" || s[2].Relation != OPBLessEqual {
		t.Fatal("relations", s)
	}
}

func TestParseOPBPreservesMaxSyntaxAndEquality(t *testing.T) {
	document, err := ParseOPB(strings.NewReader("max: 10 x1 +3 x2;\n1 x1 = 1;"))
	if err != nil {
		t.Fatal(err)
	}
	s := document.Statements
	if len(s) != 2 || !s[0].Objective || s[0].MinimizeSyntax || s[1].Relation != OPBEqual {
		t.Fatal(document)
	}
}

// The fork's reader consumes the "m" once; "mix1" then reads as the
// coefficient-less term x1 of a constraint (FORK FIX #39).
func TestParseOPBMPrefixFallsBackToConstraint(t *testing.T) {
	document, err := ParseOPB(strings.NewReader("mix1 >= 1;"))
	if err != nil {
		t.Fatal(err)
	}
	s := document.Statements
	if len(s) != 1 || s[0].Objective || s[0].Terms[0].Coefficient != "0" || s[0].Terms[0].Literals[0].Variable != 1 {
		t.Fatalf("%#v", document)
	}
}
