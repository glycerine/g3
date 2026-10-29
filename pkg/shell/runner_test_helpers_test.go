package shell_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/shell"
)

func runShellFixture(t *testing.T, args []string, input string) (*shell.Runner, *bytes.Buffer) {
	t.Helper()
	var output bytes.Buffer
	runner, err := shell.NewRunner(shell.RunnerConfig{Args: args, Input: strings.NewReader(input), Output: &output})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := runner.Close(); err != nil {
			t.Error(err)
		}
	})
	return runner, &output
}

func runnerMPS() string {
	pair := func(column, row string, value float64) string {
		return fmt.Sprintf("    %-8s  %-8s  %.12g\n", column, row, value)
	}
	return "NAME RUNNER\nROWS\n N COST\n L LIMIT\nCOLUMNS\n" +
		pair("x", "COST", 2) + pair("x", "LIMIT", 1) +
		"RHS\n" + pair("RHS1", "LIMIT", 5) + "ENDATA\n"
}
