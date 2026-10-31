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
	log := "R\nC 3\n= 0x42\nR\nP 0x42\nS \"proof\"\nS \"true\"\nC 5\nR\nP 0x42\nC 6\n= 0x43\nR\nP 0x43\nC 13\n= 0x46\nR\nP 0x43\nP 0x46\n$ |random_seed|\nU 7\nC 17\nR\nP 0x43\nC 54\n= 0x44\nR\nP 0x43\nP 0x44\nC 9\nR\nP 0x43\nP 0x44\nC 58\n= 0x45\nR\nP 0x43\nP 0x44\nC 10\nR\nP 0x43\nS \"cli-symbol\"\nC 30\nR\nP 0x43\nI 3\nC 29\nR\nP 0x42\nC 4\nR\nP 0x43\nC 8\nR\nU 0\nU 0\nU 0\nU 0\nC 342\nR\nS \"timeout\"\nS \"19\"\nC 0\nM \"replayed\"\nR\nC 1\n"
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
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader("R\nC 21\n"), &out, &diagnostics); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diagnostics.String(), "invalid command") || !strings.Contains(out.String(), "max. heap size:") {
		t.Fatal("missing command report", out.String(), diagnostics.String())
	}
}
