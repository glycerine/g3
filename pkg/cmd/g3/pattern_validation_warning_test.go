package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// AUDIT.md parsers_shell_01-F16: pattern_validator::process walks the
// pattern with for_each_expr, post-order, visiting a shared node once
// (warning text from build/z3).
func TestPatternValidatorWarnsOncePerSharedNode(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("test_vectors", "pattern_shared_forbidden.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	warnings := diagnostic.BorrowProcessWarningService()
	previous := warnings.WarningStream()
	var diagnostics bytes.Buffer
	if err = warnings.SetWarningStream(&diagnostics); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = warnings.SetWarningStream(previous) }()
	var out bytes.Buffer
	if err = run(bytes.NewReader(script), &out); err != nil {
		t.Fatal(err)
	}
	want := "WARNING: (3,178): 'and' cannot be used in patterns.\n" +
		"WARNING: (3,178): 'not' cannot be used in patterns.\n" +
		"WARNING: (3,178): 'or' cannot be used in patterns.\n"
	if out.String() != "sat\n" || diagnostics.String() != want {
		t.Fatalf("output %q, warnings %q", out.String(), diagnostics.String())
	}
}
