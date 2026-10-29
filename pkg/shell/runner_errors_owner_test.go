package shell_test

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/shell"
)

func TestRunnerRejectsAmbiguousMissingAndUnsupportedInputs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "input file was not specified"},
		{[]string{"-in", "x.smt2"}, "using standard input"},
		{[]string{"-wcnf", "-in"}, "WCNF is not ported"},
	} {
		runner, _ := runShellFixture(t, tc.args, "")
		if err := runner.Run(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("args %v error %v", tc.args, err)
		}
		result, err := runner.Result()
		if err != nil || !result.Completed || result.Error == nil {
			t.Fatalf("result %#v, error %v", result, err)
		}
	}
}

func TestRunnerBorrowsContextAndRunsOnlyOnce(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer context.Close()
	var output strings.Builder
	runner, err := shell.NewRunner(shell.RunnerConfig{
		Args: []string{"-in"}, Input: strings.NewReader("(check-sat)"), Output: &output, Context: context,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = runner.Run(); err != nil {
		t.Fatal(err)
	}
	if err = runner.Run(); err == nil {
		t.Fatal("runner accepted second execution")
	}
	if err = runner.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = context.Compliant(); err != nil {
		t.Fatal("borrowed context closed", err)
	}
}
