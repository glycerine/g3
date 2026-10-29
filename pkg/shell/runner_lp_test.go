package shell_test

import (
	"os"
	"strings"
	"testing"
)

func TestRunnerDispatchesMPSFileByExtension(t *testing.T) {
	name := t.TempDir() + "/small.mps"
	if err := os.WriteFile(name, []byte(runnerMPS()), 0o600); err != nil {
		t.Fatal(err)
	}
	runner, output := runShellFixture(t, []string{name}, "")
	if err := runner.Run(); err != nil {
		t.Fatal(err, output.String())
	}
	if !strings.Contains(output.String(), "status is OPTIMAL") || !strings.Contains(output.String(), "x = 5") {
		t.Fatalf("output %q", output.String())
	}
}
