package smt2

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2ParseAssertionsChecksAssumptionSyntaxWithoutSolving(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	if err = ParseAssertions(context, strings.NewReader("(declare-const a Bool)(check-sat a)(assert a)")); err != nil {
		t.Fatal(err)
	}
	assertions, err := context.Assertions()
	if err != nil || len(assertions) != 1 {
		t.Fatal(err, assertions)
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
}
