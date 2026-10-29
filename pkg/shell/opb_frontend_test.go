package shell

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func opbFrontendContext(t *testing.T) *cmd_context.Context {
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

func TestOPBFrontendOptimizesAndWritesResult(t *testing.T) {
	var output bytes.Buffer
	f, err := NewOPBFrontend(opbFrontendContext(t), &output, OPBFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	input := "min: 1 x1 +2 x2;\n1 x1 +1 x2 >= 1;\n"
	if err = f.Run(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	status, err := f.Status()
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	if text := output.String(); !strings.HasPrefix(text, "sat\n(objectives\n") || !strings.Contains(text, " 1)") {
		t.Fatalf("output = %q", text)
	}
	summary, _ := f.Summary()
	if summary != (cmd_context.OPBLoadSummary{Variables: 2, Constraints: 1, Objectives: 1}) {
		t.Fatal(summary)
	}
}

func TestOPBFrontendHardUnsat(t *testing.T) {
	var output bytes.Buffer
	f, err := NewOPBFrontend(opbFrontendContext(t), &output, OPBFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Run(strings.NewReader("1 x1 >= 1; 1 ~x1 >= 1;")); err != nil {
		t.Fatal(err)
	}
	if output.String() != "unsat\n" {
		t.Fatal(output.String())
	}
}
