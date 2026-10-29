package shell_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/shell"
)

func TestOpenSMTLIBFrontendRunsAndOwnsFile(t *testing.T) {
	directory := t.TempDir()
	name := filepath.Join(directory, "input.smt2")
	if err := os.WriteFile(name, []byte("(assert true)(check-sat)"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	frontend, err := shell.OpenSMTLIBFrontend(name, shell.SMTLIBFileFrontendConfig{Output: &output})
	if err != nil {
		t.Fatal(err)
	}
	if err = frontend.Run(); err != nil {
		t.Fatal(err)
	}
	if err = frontend.Close(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "sat" {
		t.Fatalf("output %q", output.String())
	}
	if _, err = frontend.Context(); err == nil {
		t.Fatal("closed frontend exposed context")
	}
}

func TestOpenSMTLIBFrontendReportsSourceFilename(t *testing.T) {
	name := filepath.Join(t.TempDir(), "missing.smt2")
	_, err := shell.OpenSMTLIBFrontend(name, shell.SMTLIBFileFrontendConfig{Output: &strings.Builder{}})
	if err == nil || !strings.Contains(err.Error(), "failed to open file '") || !strings.Contains(err.Error(), name) {
		t.Fatalf("error %v", err)
	}
}
