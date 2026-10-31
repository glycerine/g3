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

func TestZ3LogFrontendReplaysMultipleFixedpointRelationAnswer(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |P|\nU 0\np 0\nP 0x42\nC 49\n= 0x43\n" +
		"R\nP 0x41\n$ |Q|\nU 0\np 0\nP 0x42\nC 49\n= 0x44\nR\nP 0x41\nC 511\n= 0x45\nR\nP 0x41\nP 0x45\nC 512\n" +
		"R\nP 0x41\nP 0x45\nP 0x43\nC 526\nR\nP 0x41\nP 0x45\nP 0x44\nC 526\nR\nP 0x41\nP 0x45\nP 0x43\nU 0\nu 0\nC 515\n" +
		"R\nP 0x41\nP 0x45\nU 2\nP 0x43\nP 0x44\np 2\nC 518\nR\nP 0x41\nP 0x45\nC 519\n= 0x46\nR\nP 0x41\nP 0x46\nC 329\nR\nP 0x41\nP 0x45\nC 513\nM \"multiple-answer-replayed\"\n"
	var out, diagnostics bytes.Buffer
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader(log), &out, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || !strings.HasPrefix(out.String(), "multiple-answer-replayed\nmax. heap size:") {
		t.Fatal(out.String(), diagnostics.String())
	}
}
