package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

// AUDIT.md muz_rel_01-F1: rel_context::saturate opens the print_aig file
// (std::ofstream truncates it), runs the exporter and calls exit(0). The
// stream is never flushed, so z3 4.7.1 leaves the file empty for small
// outputs; no answer, later command or -st statistics are printed.
func TestDatalogPrintAIGExitsAfterTruncatingFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "out.aag")
	if err := os.WriteFile(target, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `(echo "before")
(set-option :fixedpoint.engine datalog)
(set-option :fixedpoint.print_aig "` + target + `")
(declare-rel p (Bool))
(declare-rel q (Bool))
(declare-var x Bool)
(rule (p true))
(rule (=> (p x) (q x)))
(query q)
(echo "after")
`
	var out bytes.Buffer
	err := runArguments([]string{"-st", "-in"}, strings.NewReader(script), &out)
	var exit *z3exception.ProcessExit
	if !errors.As(err, &exit) || exit.Status != 0 {
		t.Fatal("print_aig did not request exit(0)", err)
	}
	if out.String() != "before\n" {
		t.Fatalf("output %q, want %q", out.String(), "before\n")
	}
	info, err := os.Stat(target)
	if err != nil || info.Size() != 0 {
		t.Fatal("print_aig file was not truncated", info, err)
	}
}
