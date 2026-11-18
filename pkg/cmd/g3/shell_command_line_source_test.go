package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/shell"
)

// AUDIT.md parsers_shell_01-F18: parse_cmd_line_args cuts arg at its
// colon, so the message names only the option.
func TestCommandLineInvalidOptionTruncatesAtColon(t *testing.T) {
	var out bytes.Buffer
	err := runArguments([]string{"-foo:bar", "x.smt2"}, strings.NewReader(""), &out)
	var commandError *shell.CommandLineError
	if !errors.As(err, &commandError) || commandError.Message != "invalid command line option: -foo" {
		t.Fatalf("error %v", err)
	}
}

// AUDIT.md parsers_shell_01-F13: a fast DIMACS run still reports
// :total-time 0.00; only an exact 0.0 is dropped.
func TestDIMACSStatisticsIncludeTotalTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.cnf")
	if err := os.WriteFile(path, []byte("p cnf 2 2\n1 2 0\n-1 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runArguments([]string{"-st", path}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), " :total-time ") {
		t.Fatalf("statistics without total time:\n%s", out.String())
	}
}
