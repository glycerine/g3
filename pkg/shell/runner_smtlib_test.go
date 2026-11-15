package shell_test

import (
	"bytes"
	"strings"
	"sync"
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

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// AUDIT.md parsers_shell_01-F7: the global timeout ends the SMT-LIB run
// (smtlib_frontend.cpp on_timeout: display_statistics(); exit(0)), so
// nothing after "timeout" is printed and later commands do not run.
func TestSMTLIBFrontendGlobalTimeoutStopsParsing(t *testing.T) {
	var output lockedBuffer
	timer := shell.NewTimeoutService(&output)
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	script := "(declare-const a Int)(declare-const b Int)(declare-const c Int)\n" +
		"(assert (= (+ (* a a a) (* b b b)) (* c c c)))\n" +
		"(assert (> a 1000))(assert (> b 1000))\n" +
		"(check-sat)\n(echo \"after\")\n(check-sat)\n"
	frontend, err := shell.NewSMTLIBFrontend(shell.SMTLIBFrontendConfig{Input: strings.NewReader(script), Output: &output, Diagnostics: &output, Timer: timer})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := frontend.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if err = timer.SetTimeout(1); err != nil {
		t.Fatal(err)
	}
	if err = frontend.Run(); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "timeout\n" {
		t.Fatalf("output %q", got)
	}
}
