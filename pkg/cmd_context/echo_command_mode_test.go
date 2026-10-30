package cmd_context

import (
	"bytes"
	"testing"
)

func TestWriteEchoUsesRawAndCompliantSourceRepresentations(t *testing.T) {
	context, err := NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = context.WriteEcho(&output, "a\"b"); err != nil || output.String() != "a\"b\n" {
		t.Fatal(err, output.String())
	}
	if err = context.SetBoolOption("smtlib2_compliant", true); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err = context.WriteEcho(&output, "a\"b"); err != nil || output.String() != "\"a\"\"b\"\n" {
		t.Fatal(err, output.String())
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
}
