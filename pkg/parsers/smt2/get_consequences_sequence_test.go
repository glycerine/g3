package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestGetConsequencesReplacesPriorQueryAcrossScopes(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	var out bytes.Buffer
	script := `(set-logic QF_LIA)(declare-const x Int)(assert (= x 1))(get-consequences () (x))(push)(assert (= x 2))(get-consequences () (x))(pop)(get-consequences () (x))`
	if err = ParseCommands(c, strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	want := "sat\n\n(=> true (= x 1))\nunsat\n\nsat\n\n(=> true (= x 1))\n"
	if out.String() != want {
		t.Fatal(out.String(), want)
	}
}
