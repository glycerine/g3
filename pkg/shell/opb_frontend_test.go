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

// AUDIT.md FUZZ_A-F1: display_results prints get_lower/get_upper after an
// unsat initial check: the maxsmt bounds 0..sum of the weights.
func TestOPBFrontendUnsatInitialMaxSMTBounds(t *testing.T) {
	var output bytes.Buffer
	f, err := NewOPBFrontend(opbFrontendContext(t), &output, OPBFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Run(strings.NewReader("min: +1 x1 +2 x2 +4 x3 ;\n+1 x1 +1 x2 +1 x3 >= 2 ;\n-1 x1 -1 x2 -1 x3 >= -1 ;\n")); err != nil {
		t.Fatal(err)
	}
	if output.String() != "unsat\n  [0:7]\n" {
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
	// opt_frontend display_statistics: opt::context::collect_statistics as
	// plain "name: value" lines, then "time: ... secs", then the objective
	// value (AUDIT.md FUZZ_J-F6).
	text := output.String()
	var keys []string
	for _, line := range strings.Split(text, "\n") {
		if i := strings.Index(line, ":"); i > 0 {
			keys = append(keys, line[:i])
		}
	}
	want := []string{"binary propagations", "decisions", "max memory", "memory", "mk binary clause", "mk bool var", "num allocs", "rlimit count", "time"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("statistics keys %q, want %q in %q", keys, want, text)
	}
	timeLine, value := strings.Index(text, "\ntime:                "), strings.LastIndex(text, " secs\n   1\n")
	if timeLine < 0 || value < timeLine || strings.Contains(text, ":total-time") {
		t.Fatalf("statistics and objective order: %q", text)
	}
}
