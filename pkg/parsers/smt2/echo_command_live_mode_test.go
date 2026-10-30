package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2EchoReadsComplianceModeForEveryCommand(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader("(echo \"first\")"), &output); err != nil {
		t.Fatal(err)
	}
	if err = context.SetBoolOption("smtlib2_compliant", true); err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	if err = ParseCommands(context, strings.NewReader("(echo \"second\")"), &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "first\n\"second\"\n" {
		t.Fatal(output.String())
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
}
