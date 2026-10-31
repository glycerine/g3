package shell

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func wcnfFrontendContext(t *testing.T) *cmd_context.Context {
	t.Helper()
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	return c
}

func TestWCNFFrontendOptimizesWeightedSoftClauses(t *testing.T) {
	var output bytes.Buffer
	f, err := NewWCNFFrontend(wcnfFrontendContext(t), &output, WCNFFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	input := "p wcnf 1 3 10\n10 1 0\n2 -1 0\n3 1 0\n"
	if err = f.Run(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	status, err := f.Status()
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	if text := output.String(); text != "sat\n   2\n" {
		t.Fatalf("output=%q", text)
	}
	summary, _ := f.Summary()
	if summary != (cmd_context.WCNFLoadSummary{Variables: 1, HardClauses: 1, SoftClauses: 2}) {
		t.Fatal(summary)
	}
}

func TestWCNFFrontendHardUnsatAndLifecycle(t *testing.T) {
	var output bytes.Buffer
	f, err := NewWCNFFrontend(wcnfFrontendContext(t), &output, WCNFFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Status(); err != ErrWCNFFrontend {
		t.Fatal(err)
	}
	input := "p wcnf 1 2 4\n4 1 0\n4 -1 0\n"
	if err = f.Run(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if output.String() != "unsat\n" {
		t.Fatal(output.String())
	}
	if err = f.Run(strings.NewReader(input)); err != ErrWCNFFrontend {
		t.Fatal(err)
	}
}
