package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestGetConsequencesUsesOnlyRequiredAntecedents(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	var out bytes.Buffer
	script := `(set-logic QF_LIA)(declare-const a Bool)(declare-const b Bool)(declare-const x Int)(assert (=> a (= x 7)))(get-consequences (a b) (x))`
	if err = ParseCommands(c, strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if want := "sat\n\n(=> a (= x 7))\n"; out.String() != want {
		t.Fatal(out.String(), want)
	}
}
