package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestGetConsequencesEmptySelectionPreservesStatus(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	var out bytes.Buffer
	if err = ParseCommands(c, strings.NewReader(`(set-logic QF_UF)(get-consequences () ())(assert false)(get-consequences () ())`), &out); err != nil {
		t.Fatal(err)
	}
	if want := "sat\n\nunsat\n\n"; out.String() != want {
		t.Fatal(out.String(), want)
	}
}
