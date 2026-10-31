package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/api"
)

func TestZ3LogFrontendCLIInputAndFile(t *testing.T) {
	t.Cleanup(func() {
		if err := api.GlobalParamResetAll(); err != nil {
			t.Error(err)
		}
	})
	log := "R\nC 3\n= 0x42\nR\nP 0x42\nS \"proof\"\nS \"true\"\nC 5\nR\nP 0x42\nC 6\n= 0x43\nR\nP 0x43\nC 13\n= 0x46\nR\nP 0x43\nP 0x46\n$ |random_seed|\nU 7\nC 17\nR\nP 0x43\nC 33\n= 0x50\nR\nP 0x43\nP 0x50\nP 0x50\nC 37\n= 0x56\nR\nP 0x43\nI 3\nP 0x50\nC 150\n= 0x51\nR\nP 0x43\nP 0x51\nP 0x51\nC 73\n= 0x52\nR\nP 0x43\nC 32\n= 0x48\nR\nP 0x43\n$ |ReplayList|\nP 0x48\nP 0\nP 0\nP 0\nP 0\nP 0\nP 0\nC 41\n= 0x57\n* 0x58 3\nR\nP 0x43\n$ |cli-variable|\nP 0x48\nC 51\n= 0x49\nR\nP 0x43\nC 54\n= 0x44\nR\nP 0x43\nP 0x44\nC 9\nR\nP 0x43\nP 0x44\nC 58\n= 0x45\nR\nP 0x43\nU 2\nP 0x44\nP 0x45\np 2\nC 63\n= 0x47\nR\nP 0x43\nP 0x44\nC 10\nR\nP 0x43\nS \"cli-symbol\"\nC 30\nR\nP 0x43\nI 3\nC 29\nR\nP 0x43\nC 408\n= 0x53\nR\nP 0x43\nP 0x53\nC 416\nR\nP 0x43\nP 0x53\nC 414\n= 0x62\nR\nP 0x43\nP 0x62\nC 25\nR\nP 0x43\nP 0x53\nP 0x49\nC 422\nR\nP 0x43\nP 0x53\nC 427\nR\nP 0x43\nP 0x53\nC 431\n= 0x54\nR\nP 0x43\nP 0x54\nP 0x49\nI 0\nP 0\nC 298\n* 0x55 4\nR\nP 0x43\nP 0x53\nC 420\nR\nP 0x43\nP 0x53\nC 427\nR\nP 0x43\nP 0x53\nC 417\nR\nP 0x43\nU 8\nC 35\n= 0x59\nR\nP 0x43\nU 5\nP 0x59\nC 151\n= 0x60\nR\nP 0x43\nP 0x60\nP 0x60\nC 90\n= 0x61\nR\nP 0x42\nC 4\nR\nP 0x43\nC 8\nR\nU 0\nU 0\nU 0\nU 0\nC 342\nR\nS \"timeout\"\nS \"19\"\nC 0\nM \"replayed\"\nR\nC 1\n"
	for _, file := range []bool{false, true} {
		var args []string
		if file {
			path := filepath.Join(t.TempDir(), "source.log")
			if err := os.WriteFile(path, []byte(log), 0600); err != nil {
				t.Fatal(err)
			}
			args = []string{path}
		} else {
			args = []string{"-log", "-in"}
		}
		var out, diagnostics bytes.Buffer
		if err := runArgumentsWithDiagnostics(args, strings.NewReader(log), &out, &diagnostics); err != nil {
			t.Fatal(err)
		}
		if diagnostics.Len() != 0 || !strings.HasPrefix(out.String(), "replayed\nmax. heap size:") || !strings.Contains(out.String(), "\ntime: ") {
			t.Fatal("log frontend", out.String(), diagnostics.String())
		}
	}
}

func TestZ3LogFrontendReportsUnavailableCommands(t *testing.T) {
	var out, diagnostics bytes.Buffer
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader("R\nC 488\n"), &out, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diagnostics.String(), "invalid command") || !strings.Contains(out.String(), "max. heap size:") {
		t.Fatal("missing command report", out.String(), diagnostics.String())
	}
}

func TestZ3LogFrontendReplaysQuantifier(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\nU 0\nP 0x42\nC 191\n= 0x43\n" +
		"R\nP 0x41\nU 1\nU 0\np 0\nU 1\nP 0x42\np 1\n$ |b|\ns 1\nP 0x43\nC 192\n= 0x44\n" +
		"M \"quantifier-replayed\"\n"
	var out, diagnostics bytes.Buffer
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader(log), &out, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || !strings.HasPrefix(out.String(), "quantifier-replayed\nmax. heap size:") {
		t.Fatal(out.String(), diagnostics.String())
	}
}

func TestZ3LogFrontendReplaysConstantQuantifier(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\n# 37\nP 0x42\nC 51\n= 0x43\n" +
		"R\nP 0x41\nU 1\nU 1\nP 0x43\np 1\nU 0\np 0\nP 0x43\nC 196\n= 0x44\n" +
		"R\nP 0x41\nP 0x44\nC 277\nR\nP 0x41\nP 0x44\nC 278\nR\nP 0x41\nP 0x44\nC 279\nR\nP 0x41\nP 0x44\nC 281\n" +
		"R\nP 0x41\nP 0x44\nC 283\nR\nP 0x41\nP 0x44\nU 0\nC 284\nR\nP 0x41\nP 0x44\nU 0\nC 285\n= 0x45\nR\nP 0x41\nP 0x44\nC 286\n= 0x46\n" +
		"M \"constant-quantifier-replayed\"\n"
	var out, diagnostics bytes.Buffer
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader(log), &out, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || !strings.HasPrefix(out.String(), "constant-quantifier-replayed\nmax. heap size:") {
		t.Fatal(out.String(), diagnostics.String())
	}
}

func TestZ3LogFrontendReplaysSimplification(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 54\n= 0x42\n" +
		"R\nP 0x41\nP 0x42\nC 287\n= 0x43\n" +
		"R\nP 0x41\nC 289\nR\nP 0x41\nC 290\n= 0x44\n" +
		"R\nP 0x41\nP 0x44\nC 22\nR\nP 0x41\nP 0x44\nC 23\n" +
		"M \"simplification-replayed\"\n"
	var out, diagnostics bytes.Buffer
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader(log), &out, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || !strings.HasPrefix(out.String(), "simplification-replayed\nmax. heap size:") {
		t.Fatal(out.String(), diagnostics.String())
	}
}

func TestZ3LogFrontendReplaysSequencesAndRegex(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 159\n= 0x42\n" +
		"R\nP 0x41\nS \"hello\"\nC 161\n= 0x43\nR\nP 0x41\nP 0x43\nC 177\n= 0x44\n" +
		"R\nP 0x41\nP 0x44\nU 1\nU 0\nC 185\n= 0x45\nR\nP 0x41\nP 0x43\nP 0x45\nC 178\n= 0x46\n" +
		"M \"sequence-replayed\"\n"
	var out, diagnostics bytes.Buffer
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader(log), &out, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || !strings.HasPrefix(out.String(), "sequence-replayed\nmax. heap size:") {
		t.Fatal(out.String(), diagnostics.String())
	}
}

func TestZ3LogFrontendReplaysGoal(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nI 1\nI 0\nI 0\nC 348\n= 0x42\n" +
		"R\nP 0x41\nP 0x42\nC 349\nR\nP 0x41\nC 55\n= 0x43\n" +
		"R\nP 0x41\nP 0x42\nP 0x43\nC 352\nR\nP 0x41\nP 0x42\nC 353\n" +
		"R\nP 0x41\nP 0x42\nU 0\nC 357\n= 0x44\nR\nP 0x41\nP 0x42\nC 350\nM \"goal-replayed\"\n"
	var out, diagnostics bytes.Buffer
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader(log), &out, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || !strings.HasPrefix(out.String(), "goal-replayed\nmax. heap size:") {
		t.Fatal(out.String(), diagnostics.String())
	}
}

func TestZ3LogFrontendReplaysASTVector(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 447\n= 0x42\nR\nP 0x41\nP 0x42\nC 448\n" +
		"R\nP 0x41\nC 54\n= 0x43\nR\nP 0x41\nP 0x42\nP 0x43\nC 454\n" +
		"R\nP 0x41\nP 0x42\nU 0\nC 451\n= 0x44\nR\nP 0x41\nP 0x42\nC 449\nM \"vector-replayed\"\n"
	var out, diagnostics bytes.Buffer
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader(log), &out, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || !strings.HasPrefix(out.String(), "vector-replayed\nmax. heap size:") {
		t.Fatal(out.String(), diagnostics.String())
	}
}

func TestZ3LogFrontendReplaysTacticApplication(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nI 0\nI 0\nI 0\nC 348\n= 0x42\nR\nP 0x41\nP 0x42\nC 349\n" +
		"R\nP 0x41\nC 377\n= 0x43\nR\nP 0x41\nP 0x43\nC 364\nR\nP 0x41\nP 0x43\nP 0x42\nC 400\n= 0x44\n" +
		"R\nP 0x41\nP 0x44\nC 402\nR\nP 0x41\nP 0x44\nU 0\nC 406\n= 0x45\nR\nP 0x41\nP 0x44\nC 403\nM \"tactic-replayed\"\n"
	var out, diagnostics bytes.Buffer
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader(log), &out, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || !strings.HasPrefix(out.String(), "tactic-replayed\nmax. heap size:") {
		t.Fatal(out.String(), diagnostics.String())
	}
}

func TestZ3LogFrontendReplaysASTMap(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 457\n= 0x42\nR\nP 0x41\nP 0x42\nC 458\n" +
		"R\nP 0x41\nC 54\n= 0x43\nR\nP 0x41\nC 55\n= 0x44\n" +
		"R\nP 0x41\nP 0x42\nP 0x43\nP 0x44\nC 462\nR\nP 0x41\nP 0x42\nP 0x43\nC 461\n= 0x45\n" +
		"R\nP 0x41\nP 0x42\nC 459\nM \"map-replayed\"\n"
	var out, diagnostics bytes.Buffer
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader(log), &out, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || !strings.HasPrefix(out.String(), "map-replayed\nmax. heap size:") {
		t.Fatal(out.String(), diagnostics.String())
	}
}

func TestZ3LogFrontendReplaysFixedpoint(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 511\n= 0x42\nR\nP 0x41\nP 0x42\nC 512\n" +
		"R\nP 0x41\nP 0x42\nC 536\nR\nP 0x41\nP 0x42\nC 537\nR\nP 0x41\nP 0x42\nC 528\n= 0x43\n" +
		"R\nP 0x41\nP 0x42\nC 513\nM \"fixedpoint-replayed\"\n"
	var out, diagnostics bytes.Buffer
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader(log), &out, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || !strings.HasPrefix(out.String(), "fixedpoint-replayed\nmax. heap size:") {
		t.Fatal(out.String(), diagnostics.String())
	}
}
