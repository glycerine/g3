package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestZ3LogFrontendReplaysFixedpointProgramLoading(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 511\n= 0x42\nR\nP 0x41\nP 0x42\nC 512\n" +
		"R\nP 0x41\nP 0x42\nS \"(declare-rel R ()) (rule R fact) (query R)\"\nC 534\n= 0x43\nR\nP 0x41\nP 0x43\nC 448\n" +
		"R\nP 0x41\nP 0x43\nU 0\nC 451\n= 0x44\nR\nP 0x41\nP 0x42\nP 0x44\nC 517\n" +
		"R\nP 0x41\nP 0x43\nC 449\nR\nP 0x41\nP 0x42\nC 513\nM \"fixedpoint-load-replayed\"\n"
	var out, diagnostics bytes.Buffer
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader(log), &out, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || !strings.HasPrefix(out.String(), "fixedpoint-load-replayed\nmax. heap size:") {
		t.Fatal(out.String(), diagnostics.String())
	}
}
