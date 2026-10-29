package shell_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/shell"
)

func newBorrowedFrontend(t *testing.T, script string, statistics bool, clock func() (uint64, error)) (*shell.SMTLIBFrontend, *cmd_context.Context, *bytes.Buffer) {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	var output bytes.Buffer
	frontend, err := shell.NewSMTLIBFrontend(shell.SMTLIBFrontendConfig{
		Context: context, Input: strings.NewReader(script), Output: &output,
		DisplayStatistics: statistics, Clock: clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := frontend.Close(); err != nil {
			t.Error(err)
		}
	})
	return frontend, context, &output
}

func steppedClock(values ...uint64) func() (uint64, error) {
	n := 0
	return func() (uint64, error) {
		value := values[n]
		n++
		return value, nil
	}
}
