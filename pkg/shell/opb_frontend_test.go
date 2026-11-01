package shell

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/opt"
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
	if text := output.String(); text != "sat\n   1\n" {
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

func TestOPBFrontendUnsatObjectiveBounds(t *testing.T) {
	var output bytes.Buffer
	f, err := NewOPBFrontend(opbFrontendContext(t), &output, OPBFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Run(strings.NewReader("min: 2 x1; 1 x1 >= 1; 1 ~x1 >= 1;")); err != nil {
		t.Fatal(err)
	}
	if output.String() != "unsat\n  [(* (- 1) oo):oo]\n" {
		t.Fatalf("unsat bounds: %q", output.String())
	}
}

func TestOPBFrontendUnknownObjectiveBounds(t *testing.T) {
	var output bytes.Buffer
	c := opbFrontendContext(t)
	f, err := NewOPBFrontend(c, &output, OPBFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	document, err := opt.ParseOPB(strings.NewReader("min: 2 x1; 1 x1 >= 1;"))
	if err != nil {
		t.Fatal(err)
	}
	if f.summary, err = c.LoadOPB(document); err != nil {
		t.Fatal(err)
	}
	f.finished = true
	if err = f.WriteResult(); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "unknown\n  [(* (- 1) oo):oo]\n"; got != want {
		t.Fatalf("unknown bounds = %q, want %q", got, want)
	}
}

func TestOPBFrontendUnknownUsesAvailableBounds(t *testing.T) {
	var output bytes.Buffer
	f, err := NewOPBFrontend(opbFrontendContext(t), &output, OPBFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Run(strings.NewReader("min: 2 x1; 1 x1 >= 1;")); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	f.status = lbool.Undef
	if err = f.WriteResult(); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "unknown\n   2\n"; got != want {
		t.Fatalf("unknown bounds = %q, want %q", got, want)
	}
}

func TestOPBFrontendStatisticsPrecedeObjective(t *testing.T) {
	var output bytes.Buffer
	f, err := NewOPBFrontend(opbFrontendContext(t), &output, OPBFrontendOptions{DisplayStatistics: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Run(strings.NewReader("min: 1 x1 +2 x2; 1 x1 +1 x2 >= 1;")); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	statistics, value := strings.Index(text, ":total-time"), strings.LastIndex(text, "   1\n")
	if statistics < 0 || value < statistics {
		t.Fatalf("statistics and objective order: %q", text)
	}
}
