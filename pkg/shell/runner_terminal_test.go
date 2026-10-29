package shell_test

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/shell"
)

func TestRunnerTerminalActionsDoNotRequireInputSelection(t *testing.T) {
	for _, tc := range []struct {
		arg, contains string
		action        shell.CommandAction
	}{
		{"-h", "Usage: z3", shell.CommandHelp},
		{"-version", "Z3 version", shell.CommandVersion},
		{"-pm", "Use -pm:name", shell.CommandDisplayModules},
	} {
		runner, output := runShellFixture(t, []string{tc.arg}, "")
		if err := runner.Run(); err != nil {
			t.Fatal(tc.arg, err)
		}
		if !strings.Contains(output.String(), tc.contains) {
			t.Fatalf("%s output %q", tc.arg, output.String())
		}
		result, err := runner.Result()
		if err != nil || result.Action != tc.action || result.InputName != "" {
			t.Fatalf("result %#v, error %v", result, err)
		}
	}
}
