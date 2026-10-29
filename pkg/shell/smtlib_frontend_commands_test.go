package shell_test

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/shell"
)

func TestSMTLIBFrontendExecutesScopedCommandStream(t *testing.T) {
	script := `(set-logic QF_LIA)
(declare-const x Int)
(assert (= x 4))
(check-sat)
(get-value (x))
(push)
(assert (not (= x 4)))
(check-sat)
(pop)
(check-sat)
`
	frontend, _, output := newBorrowedFrontend(t, script, false, nil)
	if err := frontend.Run(); err != nil {
		t.Fatal(err, output.String())
	}
	got := output.String()
	for _, want := range []string{"sat", "((x 4))", "unsat", "sat"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output %q does not contain %q", got, want)
		}
	}
	result, err := frontend.Result()
	if err != nil || !result.Completed || result.ParseError != nil {
		t.Fatalf("result %#v, error %v", result, err)
	}
}

func TestRunSMTLIBStreamOwnsStandaloneSession(t *testing.T) {
	var output strings.Builder
	if err := shell.RunSMTLIBStream(strings.NewReader("(assert false)(check-sat)"), &output, false); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "unsat" {
		t.Fatalf("output %q", output.String())
	}
}
