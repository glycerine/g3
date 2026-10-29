package opt

import (
	"strings"
	"testing"
)

func TestParseOPBRejectsMalformedRetainedGrammar(t *testing.T) {
	cases := []string{
		"min:;",
		"min: 1 y1;",
		"1 x1 > 0;",
		"1 x1 >= 0",
		"1 x2147483648 >= 0;",
		"1 >= 0;",
		"1 x1 x >= 0;",
	}
	for _, input := range cases {
		if _, err := ParseOPB(strings.NewReader(input)); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}

func TestOPBDocumentCopyOwnsIntegersAndSlices(t *testing.T) {
	d, err := ParseOPB(strings.NewReader("min: 7 x1; 2 x1 >= 1;"))
	if err != nil {
		t.Fatal(err)
	}
	c := d.Copy()
	c.Objectives[0].Terms[0].Coefficient.SetInt64(99)
	c.Constraints[0].Terms[0].Literals[0].Variable = 8
	if d.Objectives[0].Terms[0].Coefficient.String() != "7" || d.Constraints[0].Terms[0].Literals[0].Variable != 1 {
		t.Fatal("copy aliases source")
	}
}
