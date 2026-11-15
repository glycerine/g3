package main

import (
	"bytes"
	"strings"
	"testing"
)

// replayLog runs a '/'-separated z3 log through the -log frontend.
func replayLog(t *testing.T, log string) (string, string) {
	t.Helper()
	var out, diagnostics bytes.Buffer
	text := strings.ReplaceAll(log, "/", "\n") + "\n"
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader(text), &out, &diagnostics); err != nil {
		t.Fatal(err, out.String(), diagnostics.String())
	}
	return out.String(), diagnostics.String()
}

// AUDIT.md api_03-F2: check_arg failures inside a command are
// z3_replayer_exceptions: printed as [z3 exception] and replay continues;
// outside a command ('*') they are fatal with the bare message.
func TestZ3LogFrontendCommandArgumentException(t *testing.T) {
	out, diagnostics := replayLog(t, `R/P 0/C 6/= 10/R/I 5/C 33/M "after"/R/P 10/C 8`)
	if !strings.HasPrefix(out, "[z3 exception]: expecting object at position 0 but got int64\nafter\nmax. heap size:") || diagnostics != "" {
		t.Fatalf("command argument exception: %q %q", out, diagnostics)
	}
	out, diagnostics = replayLog(t, `R/P 0/C 6/= 10/R/P 10/I 3/* 21 1`)
	if diagnostics != "Error at line 8: expecting object at position 1 but got int64\n" || !strings.HasPrefix(out, "max. heap size:") {
		t.Fatalf("save-out argument error: %q %q", out, diagnostics)
	}
}
