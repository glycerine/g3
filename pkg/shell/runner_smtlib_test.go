package shell_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"testing/iotest"
	"time"

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

// AUDIT.md parsers_shell_01-F7: the global timeout ends the SMT-LIB run
// (smtlib_frontend.cpp on_timeout: display_statistics(); exit(0)), so
// nothing after "timeout" is printed and later commands do not run.
func TestSMTLIBFrontendGlobalTimeoutStopsParsing(t *testing.T) {
	var output bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	script := "(declare-const a Int)(declare-const b Int)(declare-const c Int)\n" +
		"(assert (= (+ (* a a a) (* b b b)) (* c c c)))\n" +
		"(assert (> a 1000))(assert (> b 1000))\n" +
		"(check-sat)\n(echo \"after\")\n(check-sat)\n"
	frontend, err := shell.NewSMTLIBFrontend(shell.SMTLIBFrontendConfig{Input: strings.NewReader(script), Output: &output, Diagnostics: &output})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := frontend.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if err = frontend.RunContext(ctx); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "timeout\n" {
		t.Fatalf("output %q", got)
	}
}

func TestSMTLIBFrontendClosedRunSkipsCommandsAndTimeoutReport(t *testing.T) {
	var output bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frontend, err := shell.NewSMTLIBFrontend(shell.SMTLIBFrontendConfig{
		Input:  iotest.ErrReader(errors.New("read after run closed")),
		Output: &output, Diagnostics: &output, DisplayStatistics: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer frontend.Close()
	cancel()
	if err := frontend.RunContext(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := frontend.Result()
	if err != nil || result.ParseError != nil || output.Len() != 0 {
		t.Fatalf("closed run produced result %+v, error %v, output %q", result, err, output.String())
	}
}

func TestRunShellContextHonorsEarlierParentDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	var output bytes.Buffer
	err := shell.RunShellContext(ctx, shell.RunnerConfig{
		Args:   []string{"-T:60", "-in"},
		Input:  iotest.ErrReader(errors.New("read after parent deadline")),
		Output: &output,
	})
	if err != nil || output.String() != "timeout\n" {
		t.Fatalf("output %q, error %v", output.String(), err)
	}
}
