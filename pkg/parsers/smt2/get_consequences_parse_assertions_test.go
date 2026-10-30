package smt2

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestParseAssertionsChecksConsequencesWithoutExecuting(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	script := `(set-logic QF_LIA)(declare-const x Int)(assert (= x 4))(get-consequences () (x))(assert (> x 0))`
	if err = ParseAssertions(c, strings.NewReader(script)); err != nil {
		t.Fatal(err)
	}
	assertions, err := c.Assertions()
	if err != nil || len(assertions) != 2 {
		t.Fatal("loaded assertions", len(assertions), err)
	}
	if reason, reasonErr := c.ReasonUnknown(); reasonErr != nil || reason != "state of the most recent check-sat command is not known" {
		t.Fatal("consequence query executed while loading", reason, reasonErr)
	}
}
