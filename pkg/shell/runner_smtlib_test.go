package shell_test

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/shell"
)

func TestRunnerDispatchesSMTLIBStdinWithParametersAndStatistics(t *testing.T) {
	runner, output := runShellFixture(t, []string{"proof=true", "-st", "-in"}, "(assert false)(check-sat)(get-proof)")
	if err := runner.Run(); err != nil {
		t.Fatal(err, output.String())
	}
	got := output.String()
	if !strings.HasPrefix(got, "unsat\n") || !strings.Contains(got, "asserted") || !strings.Contains(got, ":total-time") {
		t.Fatalf("output %q", got)
	}
	result, err := runner.Result()
	if err != nil || !result.Completed || result.InputName != "stdin" || result.Kind != shell.InputSMT2 {
		t.Fatalf("result %#v, error %v", result, err)
	}
}
