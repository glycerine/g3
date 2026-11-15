package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/shell"
)

// AUDIT.md parsers_shell_01-F12: open failures of the Datalog, DIMACS and
// MPS front ends print the C++ messages; Datalog and MPS end successfully
// (main ignores read_datalog's result, run_solver just returns), DIMACS
// exits with ERR_OPEN_FILE.
func TestFrontEndOpenFailuresMatchCPP(t *testing.T) {
	dir := t.TempDir()
	missing := func(name string) string { return filepath.Join(dir, name) }
	var output, diagnostics bytes.Buffer
	if err := runArgumentsWithDiagnostics([]string{missing("nofile.dl")}, strings.NewReader(""), &output, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if want := "ERROR: could not open file '" + missing("nofile.dl") + "'.\nERROR: failed to parse file\n"; diagnostics.String() != want || output.String() != "" {
		t.Fatalf("datalog stdout %q stderr %q", output.String(), diagnostics.String())
	}
	output.Reset()
	diagnostics.Reset()
	if err := runArgumentsWithDiagnostics([]string{missing("nofile.mps")}, strings.NewReader(""), &output, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if output.String() != "setting m_is_OK to false\n" || diagnostics.String() != "cannot process "+missing("nofile.mps")+"\n" {
		t.Fatalf("mps stdout %q stderr %q", output.String(), diagnostics.String())
	}
	err := runArgumentsWithDiagnostics([]string{missing("nofile.cnf")}, strings.NewReader(""), &output, &diagnostics)
	var openError *shell.SMTLIBFileOpenError
	if !errors.As(err, &openError) || openError.Name != missing("nofile.cnf") {
		t.Fatal("dimacs open failure", err)
	}
}
