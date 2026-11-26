package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/diagnostic"
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

func TestDIMACSFileModelProbes(t *testing.T) {
	for _, tc := range []struct {
		name, clauses, model string
	}{
		{"positive", "1 -2 0\n2 0\n", "1 2 "},
		{"negative", "-1 2 0\n-2 0\n", "-1 -2 "},
		{"mixed assignment", "1 0\n-2 0\n", "1 -2 "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "model.cnf")
			source := "c " + strings.Repeat(" ", 4096) + "\np cnf 99 7\n" + tc.clauses
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := runArguments([]string{"-st", path}, strings.NewReader(""), &out); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(out.String(), "sat\n"+tc.model+"\n") {
				t.Fatalf("model output %q", out.String())
			}
			if allocated := strings.Contains(out.String(), ":mk-bool-var"); allocated != diagnostic.TraceCompiled() {
				t.Fatalf("unexpected DIMACS path:\n%s", out.String())
			}
		})
	}
}

func TestDIMACSFileUniformModelHonorsConflictLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bounded.cnf")
	source := "c " + strings.Repeat(" ", 4096) + "\np cnf 2 2\n1 0\n2 0\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runArguments([]string{"sat.max_conflicts=0", path}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "unknown\n" {
		t.Fatalf("conflict-limited output %q", out.String())
	}
}

func TestDIMACSFileModelProbesFallBackOnMalformedSuffix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "malformed.cnf")
	source := "c " + strings.Repeat(" ", 4096) + "\np cnf 3 3\n1 0\n-2 0\n3 x\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := runArguments([]string{path}, strings.NewReader(""), &out)
	if !errors.Is(err, sat.ErrDIMACSUnexpectedChar) || out.Len() != 0 {
		t.Fatalf("malformed suffix status %v, output %q", err, out.String())
	}
}
