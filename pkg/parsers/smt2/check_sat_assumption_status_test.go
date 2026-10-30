package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2DirectCheckSatValidatesExpectedStatus(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	var output bytes.Buffer
	script := "(set-info :status sat)(declare-const a Bool)(assert (not a))(check-sat a)"
	err = ParseCommands(context, strings.NewReader(script), &output)
	if err == nil || !strings.Contains(err.Error(), "check annotation that says sat") || output.String() != "unsat\n" {
		t.Fatal(err, output.String())
	}
	if closeErr := context.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
}
