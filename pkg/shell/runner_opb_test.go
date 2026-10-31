package shell_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/shell"
)

func TestRunnerDispatchesOPB(t *testing.T) {
	input := "min: 1 x1 +2 x2;\n1 x1 +1 x2 >= 1;\n"
	runner, output := runShellFixture(t, []string{"-bpo", "-in"}, input)
	if err := runner.Run(); err != nil {
		t.Fatal(err)
	}
	if text := output.String(); text != "sat\n   1\n" {
		t.Fatalf("OPB output: %q", text)
	}
	result, err := runner.Result()
	if err != nil || result.Kind != shell.InputOPB {
		t.Fatalf("OPB result: %#v, %v", result, err)
	}
}
