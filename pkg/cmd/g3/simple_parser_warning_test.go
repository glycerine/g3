package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// AUDIT.md parsers_shell_01-F17: an unparsable smt.qi.cost reaches
// simple_parser::parse, whose catch prints warning_msg("parser error"), then
// qi_queue::setup warns about the fallback; every SMT context repeats both.
func TestSimpleParserWarningPrefixMatchesReference(t *testing.T) {
	name := filepath.Join("test_vectors", "qi_cost_parser_error_warning.smt2")
	script, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	warnings := diagnostic.BorrowProcessWarningService()
	var diagnostics bytes.Buffer
	if err = warnings.SetWarningStream(&diagnostics); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := warnings.SetWarningStream(nil); err != nil {
			t.Error(err)
		}
	}()
	var out bytes.Buffer
	if err = run(bytes.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	pair := "WARNING: parser error\nWARNING: invalid cost function '(foo)', switching to default one\n"
	got := diagnostics.String()
	if out.String() != "unsat\n" || got == "" || strings.ReplaceAll(got, pair, "") != "" {
		t.Fatalf("output %q warnings %q", out.String(), got)
	}
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		return
	}
	cmd := exec.Command(reference, "-in")
	cmd.Stdin = bytes.NewReader(script)
	var native, nativeDiagnostics bytes.Buffer
	cmd.Stdout, cmd.Stderr = &native, &nativeDiagnostics
	if err = cmd.Run(); err != nil {
		t.Fatal(err, native.String(), nativeDiagnostics.String())
	}
	if native.String() != out.String() || nativeDiagnostics.String() != got {
		t.Fatalf("Go/C++ mismatch\nGo: %q\n%q\nC++: %q\n%q", out.String(), got, native.String(), nativeDiagnostics.String())
	}
}
