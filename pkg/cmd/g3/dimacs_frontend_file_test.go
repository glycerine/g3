package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDIMACSFrontendDetectsCNFFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conflict.cnf")
	if err := os.WriteFile(path, []byte("p cnf 1 2\n1 0\n-1 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runArguments([]string{path}, strings.NewReader(""), &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "unsat\n" {
		t.Fatalf("DIMACS output: %q", output.String())
	}
}
