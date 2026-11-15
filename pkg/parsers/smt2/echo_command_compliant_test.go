package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

// AUDIT.md cmd_context_01-F8: echo_cmd only wraps the string in quotes.
func TestSMT2EchoQuotesInCompliantMode(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	if err = context.SetBoolOption("smtlib2_compliant", true); err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader("(echo \"a\"\"b\")"), &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "\"a\"b\"\n" {
		t.Fatal(output.String())
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
}
