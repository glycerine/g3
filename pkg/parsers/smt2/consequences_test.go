package smt2

// Source differential: src/cmd_context/basic_cmds.cpp get-consequences.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2GetConsequencesAssumptionsVariablesAndStatuses(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	script := `(set-logic QF_LIA)(declare-const a Bool)(declare-const x Int)(declare-const y Int)
(assert (=> a (= x 3)))(assert (= y 4))
(get-consequences (a) (a x y))
(get-consequences () (a x y))
(push)(assert (not a))(get-consequences (a) (a))(pop)`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	// The order is context::get_consequences', as the C++ binary prints it.
	want := "sat\n\n(=> a a)\n(=> true (= y 4))\n(=> a (= x 3))\nsat\n\n(=> true (= y 4))\nunsat\n\n"
	if output.String() != want {
		t.Fatal("get-consequences output", output.String(), "want", want)
	}
}

func TestSMT2GetConsequencesRequiresTwoLists(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, script := range []string{`(get-consequences ())`, `(get-consequences () () extra)`} {
		var output bytes.Buffer
		if err = ParseCommands(context, strings.NewReader(script), &output); err == nil || output.Len() != 0 {
			t.Fatal("get-consequences syntax guard", script, err, output.String())
		}
	}
}
