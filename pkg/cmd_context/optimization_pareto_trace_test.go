package cmd_context

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type paretoTraceWriter struct {
	bytes.Buffer
	check func()
	fail  bool
}

func (w *paretoTraceWriter) Write(p []byte) (int, error) {
	if w.check != nil {
		w.check()
	}
	if w.fail {
		return 0, io.ErrClosedPipe
	}
	return w.Buffer.Write(p)
}

func paretoTraceFixture(t *testing.T) (*Context, *model.ModelCore, *smtcore.SMTSolver) {
	t.Helper()
	c := contextFixture(t)
	x := optimizationComparisonInteger(t, c, "pareto-trace-x")
	contextMust(t, c.AddObjective(x, true))
	md, err := model.NewModelCore(c.manager)
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, md.Close()) })
	d, err := c.manager.GetDecl(x)
	contextMust(t, err)
	v, err := c.manager.ArithmeticIntegerNumeralText("2")
	contextMust(t, err)
	contextMust(t, md.RegisterConstant(d, v))
	backend, err := c.newParetoBackend(nil)
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, backend.Close()) })
	return c, md, backend
}

func TestParetoFormulaTraceBeforeAssertionAndStreamRefusal(t *testing.T) {
	for _, dominates := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			c, md, backend := paretoTraceFixture(t)
			w := &paretoTraceWriter{fail: fail}
			w.check = func() {
				n, err := backend.NumAssertions()
				contextMust(t, err)
				if n != 0 {
					t.Fatal("trace ran after formula assertion")
				}
			}
			tags := diagnostic.NewTraceTagsWithWriter(w)
			tags.Enable("opt")
			c.config.ModelEvaluator.Trace = tags
			contextMust(t, c.assertParetoFormula(backend, md, dominates))
			n, err := backend.NumAssertions()
			contextMust(t, err)
			if n == 0 {
				t.Fatal("diagnostic stream refusal prevented assertion")
			}
			if !tags.ShouldTrace("opt") || fail {
				continue
			}
			line := "85"
			if dominates {
				line = "72"
			}
			if !strings.Contains(w.String(), "src/opt/opt_pareto.cpp:"+line) || strings.Contains(w.String(), "define-fun") != dominates {
				t.Fatal("source formula/model trace record", w.String())
			}
		}
	}
}

func TestParetoFormulaTraceOperandErrorAfterStreamFailure(t *testing.T) {
	c, md, backend := paretoTraceFixture(t)
	w := &paretoTraceWriter{fail: true}
	tags := diagnostic.NewTraceTagsWithWriter(w)
	tags.Enable("opt")
	if !tags.ShouldTrace("opt") {
		t.Skip("source TRACE erased")
	}
	c.config.ModelEvaluator.Trace = tags
	w.check = func() { c.manager.Budget().FailAfter(0) }
	err := c.assertParetoFormula(backend, md, true)
	c.manager.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("ostream refusal suppressed operand construction", err)
	}
	n, err := backend.NumAssertions()
	contextMust(t, err)
	if n != 0 {
		t.Fatal("failed diagnostic operand published formula")
	}
}

func TestParetoFormulaVerbosityThreshold(t *testing.T) {
	c, md, _ := paretoTraceFixture(t)
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	contextMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	contextMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: c.manager.Budget()})
	contextMust(t, err)
	t.Cleanup(func() {
		contextMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		contextMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		contextMust(t, stream.Close())
	})
	contextMust(t, diagnostic.SetProcessVerboseStream(stream))
	for _, dominates := range []bool{true, false} {
		ref, err := c.makeParetoFormula(md, dominates)
		contextMust(t, err)
		formula, err := c.manager.RefGet(ref)
		contextMust(t, err)
		contextMust(t, diagnostic.SetProcessVerbosityLevel(9))
		before, err := stream.String()
		contextMust(t, err)
		contextMust(t, c.paretoFormulaDiagnostics(formula, md, dominates))
		after, err := stream.String()
		contextMust(t, err)
		if after != before {
			t.Fatal("level-nine output crossed source verbosity threshold")
		}
		contextMust(t, diagnostic.SetProcessVerbosityLevel(10))
		contextMust(t, c.paretoFormulaDiagnostics(formula, md, dominates))
		contextMust(t, c.manager.DelRef(&ref))
	}
	text, err := stream.String()
	contextMust(t, err)
	if !strings.Contains(text, "dominates: ") || !strings.Contains(text, "not dominated by: ") {
		t.Fatal("missing source verbose records", text)
	}
}
