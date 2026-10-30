package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestGetConsequencesRetainsCompoundVariableExpression(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	var out bytes.Buffer
	script := `(set-logic QF_LIA)(declare-const x Int)(assert (= x 3))(get-consequences () ((+ x 1)))`
	if err = ParseCommands(c, strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if want := "sat\n\n(=> true (= (+ x 1) 4))\n"; out.String() != want {
		t.Fatal(out.String(), want)
	}
}
