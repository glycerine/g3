package shell_test

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/shell"
)

func TestRunnerDispatchesDIMACSStdin(t *testing.T) {
	runner, output := runShellFixture(t, []string{"-dimacs", "-in"}, "p cnf 2 2\n1 0\n-1 0\n")
	if err := runner.Run(); err != nil {
		t.Fatal(err, output.String())
	}
	if strings.TrimSpace(output.String()) != "unsat" {
		t.Fatalf("output %q", output.String())
	}
	result, err := runner.Result()
	if err != nil || result.Kind != shell.InputDIMACS {
		t.Fatalf("result %#v, error %v", result, err)
	}
}

func TestRunnerInputKindNamesCoverDispatchTable(t *testing.T) {
	for kind, want := range map[shell.InputKind]string{
		shell.InputSMT2: "SMT-LIB2", shell.InputDatalog: "Datalog", shell.InputDIMACS: "DIMACS",
		shell.InputWCNF: "WCNF", shell.InputOPB: "OPB", shell.InputZ3Log: "Z3 log", shell.InputMPS: "MPS",
	} {
		if got := shell.InputKindName(kind); got != want {
			t.Fatalf("kind %d: %q, want %q", kind, got, want)
		}
	}
}
