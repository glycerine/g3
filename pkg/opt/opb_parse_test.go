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
		if len(document.Constraints) != 2 || len(document.Objectives) != 1 ||
			len(document.Objectives[0].Terms) != 1 || document.Objectives[0].Terms[0].Coefficient.String() != "2" {
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
	if len(document.Objectives) != 1 || len(document.Constraints) != 2 || !document.Objectives[0].MinimizeSyntax {
		t.Fatalf("document = %#v", document)
	}
	terms := document.Objectives[0].Terms
	if len(terms) != 2 || terms[0].Coefficient.String() != "2" || terms[1].Coefficient.String() != "-3" {
		t.Fatalf("objective terms = %#v", terms)
	}
	if len(terms[1].Literals) != 2 || terms[1].Literals[0] != (OPBLiteral{Variable: 2, Negative: true}) || terms[1].Literals[1] != (OPBLiteral{Variable: 3}) {
		t.Fatalf("product = %#v", terms[1].Literals)
	}
	if document.Constraints[0].Relation != OPBGreaterEqual || document.Constraints[0].Bound.String() != "1" || document.Constraints[1].Relation != OPBLessEqual {
		t.Fatal("relations", document.Constraints)
	}
}

func TestParseOPBPreservesMaxSyntaxAndEquality(t *testing.T) {
	document, err := ParseOPB(strings.NewReader("max: 10 x1 +3 x2;\n1 x1 = 1;"))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Objectives) != 1 || document.Objectives[0].MinimizeSyntax || document.Constraints[0].Relation != OPBEqual {
		t.Fatal(document)
	}
}
