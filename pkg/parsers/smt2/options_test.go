package smt2

// Source differential: src/cmd_context/basic_cmds.cpp set/get-option commands.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2GetOptionBuiltinsAndCatalogValues(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	script := `(set-option :random-seed 17)
(set-option :produce-assignments true)
(set-option :produce-models false)
(set-option :global-declarations true)
(set-option :produce-interpolants true)
(set-option :timeout 123)
(get-option :random-seed)
(get-option :produce-assignments)
(get-option :produce-models)
(get-option :global-decls)
(get-option :produce-proofs)
(get-option :produce-interpolants)
(get-option :timeout)
(get-option :rlimit)
(get-option :smt.relevancy)
(get-option :error-behavior)
(get-option :int-real-coercions)`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	want := "17\ntrue\nfalse\ntrue\ntrue\ntrue\n123\n0\n2\ncontinued-execution\ntrue\n"
	if output.String() != want {
		t.Fatal("get-option output", output.String(), "want", want)
	}
}

func TestSMT2GetOptionUnsupportedAndSyntaxGuards(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	var output, diagnostic bytes.Buffer
	if err = context.SetDiagnosticOutput(&diagnostic); err != nil {
		t.Fatal(err)
	}
	if err = ParseCommands(context, strings.NewReader("(get-option :produce-unsat-assumptions)"), &output); err != nil {
		t.Fatal(err)
	}
	// print_unsupported: "unsupported" on the regular stream, then the note
	// on the diagnostic stream (AUDIT.md FUZZ-F6).
	if output.String() != "unsupported\n" || diagnostic.String() != "; :produce-unsat-assumptions line: 1 position: 1\n" {
		t.Fatal("get-option unsupported output", output.String(), diagnostic.String())
	}
	for _, script := range []string{`(get-option)`, `(get-option :produce-models extra)`} {
		output.Reset()
		if err = ParseCommands(context, strings.NewReader(script), &output); err == nil || output.Len() != 0 {
			t.Fatal("get-option syntax guard", script, err, output.String())
		}
	}
}

func TestSMT2NumeralAsRealResetsWithManagerState(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	script := `(set-option :numeral-as-real true)(define-const x Real 1)
(reset)(define-const y Int 1)(check-sat)`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil || output.String() != "sat\n" {
		t.Fatal("numeral-as-real reset", err, output.String())
	}
}
