package shell

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
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

func TestWCNFFrontendUnsatObjectiveBounds(t *testing.T) {
	var output bytes.Buffer
	f, err := NewWCNFFrontend(wcnfFrontendContext(t), &output, WCNFFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	input := "p wcnf 1 4 10\n10 1 0\n10 -1 0\n2 1 0\n3 -1 0\n"
	if err = f.Run(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if output.String() != "unsat\n  [0:5]\n" {
		t.Fatalf("unsat bounds: %q", output.String())
	}
}

func TestWCNFFrontendUnsatZeroWeightObjective(t *testing.T) {
	var output bytes.Buffer
	f, err := NewWCNFFrontend(wcnfFrontendContext(t), &output, WCNFFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	input := "p wcnf 1 3 10\n10 1 0\n10 -1 0\n0 1 0\n"
	if err = f.Run(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "unsat\n   0\n"; got != want {
		t.Fatalf("unsat zero-weight bounds = %q, want %q", got, want)
	}
}

func TestWCNFFrontendUnknownObjectiveBounds(t *testing.T) {
	var output bytes.Buffer
	c := wcnfFrontendContext(t)
	f, err := NewWCNFFrontend(c, &output, WCNFFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	input := "p wcnf 1 3 10\n10 1 0\n2 -1 0\n3 1 0\n"
	f.document, err = opt.ParseWCNF(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if f.summary, err = c.LoadWCNF(f.document); err != nil {
		t.Fatal(err)
	}
	f.finished = true
	if err = f.WriteResult(); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "unknown\n  [0:5]\n"; got != want {
		t.Fatalf("unknown bounds = %q, want %q", got, want)
	}
}

func TestWCNFFrontendUnknownZeroWeightObjective(t *testing.T) {
	var output bytes.Buffer
	c := wcnfFrontendContext(t)
	f, err := NewWCNFFrontend(c, &output, WCNFFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	f.document, err = opt.ParseWCNF(strings.NewReader("p wcnf 1 1 10\n0 1 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	if f.summary, err = c.LoadWCNF(f.document); err != nil {
		t.Fatal(err)
	}
	f.finished = true
	if err = f.WriteResult(); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "unknown\n   0\n"; got != want {
		t.Fatalf("unknown zero-weight bounds = %q, want %q", got, want)
	}
}

func TestWCNFFrontendUnknownUsesAvailableBounds(t *testing.T) {
	var output bytes.Buffer
	f, err := NewWCNFFrontend(wcnfFrontendContext(t), &output, WCNFFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	input := "p wcnf 1 3 10\n10 1 0\n2 -1 0\n3 1 0\n"
	if err = f.Run(strings.NewReader(input)); err != nil {
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

// AUDIT.md cmd_context_02-F2: WCNF input sets opt::context::m_is_clausal,
// so simplify_fmls returns at once: no simplifier/propagate-values/solve_eqs
// stage runs before the MaxSAT search.
func TestWCNFFrontendSkipsSimplifyFormulas(t *testing.T) {
	previous, err := diagnostic.ProcessVerbosityLevel()
	if err != nil {
		t.Fatal(err)
	}
	if err = diagnostic.SetProcessVerbosityLevel(10); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := diagnostic.SetProcessVerbosityLevel(previous); err != nil {
			t.Fatal(err)
		}
	}()
	c := wcnfFrontendContext(t)
	var output, verbose bytes.Buffer
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{BorrowedWriter: &verbose})
	if err != nil {
		t.Fatal(err)
	}
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		t.Fatal(err)
	}
	if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := diagnostic.SetProcessVerboseStream(oldStream); err != nil {
			t.Fatal(err)
		}
		if err := stream.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	f, err := NewWCNFFrontend(c, &output, WCNFFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	input := "p wcnf 4 6 10\n10 1 0\n10 -1 2 0\n3 -2 0\n2 3 4 0\n4 -3 0\n1 -4 0\n"
	if err = f.Run(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if output.String() != "sat\n   4\n" {
		t.Fatalf("output=%q", output.String())
	}
	text := verbose.String()
	first := strings.Index(text, "(card2bv")
	if first < 0 {
		t.Fatalf("no card2bv stage in %q", text)
	}
	for _, stage := range []string{"(simplifier", "(propagate-values", "(solve_eqs", "(:num-elim-vars"} {
		if i := strings.Index(text, stage); i >= 0 && i < first {
			t.Fatalf("simplify_fmls stage %s ran: %q", stage, text[:first])
		}
	}
}
