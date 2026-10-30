package shell_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/shell"
)

func TestRunnerDispatchesDatalogFileByExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reach.dl")
	if err := os.WriteFile(path, []byte("D 2\n\nP(x:D)\nP(\"a\").\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner, output := runShellFixture(t, []string{path}, "")
	if err := runner.Run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Time: ") || !strings.Contains(output.String(), "Parsing: ") {
		t.Fatalf("output %q", output.String())
	}
	result, err := runner.Result()
	if err != nil || result.Kind != shell.InputDatalog || result.InputName != path {
		t.Fatalf("result %#v error %v", result, err)
	}
}
