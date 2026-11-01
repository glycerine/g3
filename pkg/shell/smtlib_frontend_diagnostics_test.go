package shell_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/shell"
)

func TestRunnerRoutesSMTLIBDiagnosticStderr(t *testing.T) {
	script := `(set-option :parser.ignore_bad_patterns true)
(declare-fun p (Int) Bool)
(assert (forall ((x Int) (y Int)) (! (p x) :pattern ((p x)))))
(check-sat)`
	var output, diagnostics bytes.Buffer
	runner, err := shell.NewRunner(shell.RunnerConfig{Args: []string{"-in"}, Input: strings.NewReader(script), Output: &output, Diagnostics: &diagnostics})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := runner.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := runner.Run(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "sat\n" || !strings.Contains(diagnostics.String(), "pattern does not contain all quantified variables") {
		t.Fatal("shell streams", output.String(), diagnostics.String())
	}
}
