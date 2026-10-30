package shell_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/shell"
)

func TestRunnerDispatchesExplicitDatalogWPADirectory(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "V.map"), []byte("7 seven\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "p.rules"), []byte("P(x:V)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "P.rel"), []byte("7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner, _ := runShellFixture(t, []string{"-dl", directory}, "")
	if err := runner.Run(); err != nil {
		t.Fatal(err)
	}
	result, err := runner.Result()
	if err != nil || result.Kind != shell.InputDatalog {
		t.Fatalf("result %#v error %v", result, err)
	}
}
