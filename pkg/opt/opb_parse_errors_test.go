package opt

import (
	"errors"
	"strings"
	"testing"
)

func TestParseOPBRejectsMalformedRetainedGrammar(t *testing.T) {
	cases := []struct{ input, want string }{
		{"min:;", `(error line 0 "unexpected char: ;" expected "x")`},
		{"min: 1 y1;", `(error line 0 "unexpected char: y" expected "x")`},
		{"1 x1 > 0;", `(error line 0 "unexpected char: ;" expected "x")`},
		{"1 >= 0;", `(error line 0 "unexpected char: >" expected "x")`},
		{"1 x1 x >= 0;", `(error line 0 "unexpected char: >" )`},
		{"* c\n\n1 x", "(error line 2 \"unexpected char: \xff\" )"},
		{"1 x1 >= 1;\n1 x2", `(error line 1 "unexpected end of input" expected ">=", "=" or "<=")`},
	}
	for _, tc := range cases {
		_, err := ParseOPB(strings.NewReader(tc.input))
		var exit *InputParseExit
		if !errors.As(err, &exit) || exit.Status != 3 || exit.Message != tc.want {
			t.Errorf("ParseOPB(%q) = %v, want exit(3) %q", tc.input, err, tc.want)
		}
	}
}

// AUDIT.md opt_02-F4: parse_coeff_r accepts an empty digit string, which
// rational("") reads as 0, and a variable index past INT_MAX wraps.
func TestParseOPBCoefficientlessLiteral(t *testing.T) {
	d, err := ParseOPB(strings.NewReader("min: 2 x1 3 x2 ;\nx1 +1 x2 >= 1 ;\n-x3 x2147483648 = - ;\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Statements) != 3 || !d.Statements[0].Objective || len(d.Statements[0].Terms) != 2 {
		t.Fatalf("%#v", d)
	}
	terms := d.Statements[1].Terms
	if len(terms) != 2 || terms[0].Coefficient != "0" || terms[1].Coefficient != "1" {
		t.Fatalf("%#v", terms)
	}
	c := d.Statements[2]
	if c.Terms[0].Coefficient != "0" || c.Bound != "0" || c.Relation != OPBEqual || len(c.Terms[0].Literals) != 2 || c.Terms[0].Literals[1].Variable != -2147483648 {
		t.Fatalf("%#v", c)
	}
}

func TestOPBDocumentCopyOwnsIntegersAndSlices(t *testing.T) {
	d, err := ParseOPB(strings.NewReader("min: 7 x1; 2 x1 >= 1;"))
	if err != nil {
		t.Fatal(err)
	}
	c := d.Copy()
	c.Statements[0].Terms[0].Coefficient = "99"
	c.Statements[1].Terms[0].Literals[0].Variable = 8
	if d.Statements[0].Terms[0].Coefficient != "7" || d.Statements[1].Terms[0].Literals[0].Variable != 1 {
		t.Fatal("copy aliases source")
	}
}
