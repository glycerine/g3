package cmd_context

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func paretoModelVerboseStream(t *testing.T, c *Context, level uint32) *utext.NumericTextStream {
	t.Helper()
	_, err := c.Manager()
	contextMust(t, err)
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
	contextMust(t, diagnostic.SetProcessVerbosityLevel(level))
	return stream
}

func paretoModelDiagnosticFixture(t *testing.T) (*Context, *model.ModelCore, ast.ASTHandle) {
	t.Helper()
	c, x := purifiedParetoFixture(t)
	negative, err := c.manager.MkArithmeticNeg(x)
	contextMust(t, err)
	contextMust(t, c.AddObjective(negative, true))
	contextMust(t, c.prepareParetoPurification())
	md, err := model.NewModel(c.manager)
	contextMust(t, err)
	raw := md.Core()
	contextMust(t, raw.IncRef())
	t.Cleanup(func() { contextMust(t, raw.DecRef()) })
	var fresh ast.ASTHandle
	for i, term := range []ast.ASTHandle{x, c.optimizationParetoPurification.objectives[0].expression} {
		decl, err := c.manager.GetDecl(term)
		contextMust(t, err)
		text := "2"
		if i == 1 {
			text = "-2"
			fresh = decl
		}
		value, err := c.manager.ArithmeticIntegerNumeralText(text)
		contextMust(t, err)
		contextMust(t, raw.RegisterConstant(decl, value))
	}
	return c, raw, fresh
}

func TestParetoModelVerbosityGateAndFilteredCopy(t *testing.T) {
	c, raw, fresh := paretoModelDiagnosticFixture(t)
	stream := paretoModelVerboseStream(t, c, 0)
	refs := raw.References
	contextMust(t, c.paretoModelDiagnostic(raw))
	quiet, err := stream.String()
	contextMust(t, err)
	if quiet != "" {
		t.Fatal("level zero model output", quiet)
	}
	contextMust(t, diagnostic.SetProcessVerbosityLevel(1))
	contextMust(t, c.paretoModelDiagnostic(raw))
	text, err := stream.String()
	contextMust(t, err)
	if !strings.HasPrefix(text, "new model:\n") || !strings.Contains(text, "define-fun purified-x") || strings.Contains(text, "!") {
		t.Fatal("fixed model verbose output", text)
	}
	found, err := raw.HasConstantInterpretation(fresh)
	contextMust(t, err)
	if !found || raw.References != refs {
		t.Fatal("diagnostics changed raw model", found, raw.References, refs)
	}
	// An ostream badbit must not become a solver error or consume the model.
	stream.FileBad = true
	contextMust(t, c.paretoModelDiagnostic(raw))
	if raw.References != refs {
		t.Fatal("bad stream changed model ownership")
	}
}

func TestParetoDisabledModelDiagnosticDoesNotFixModel(t *testing.T) {
	c := contextFixture(t)
	paretoModelVerboseStream(t, c, 0)
	// No purification state or model exists. The disabled macro erases both
	// model fixing and model printing, including their allocations and errors.
	c.manager.Budget().FailAfter(0)
	err := c.paretoModelDiagnostic(nil)
	c.manager.Budget().FailAfter(-1)
	contextMust(t, err)
}

func TestParetoLiveModelVerbosePrecedesDominanceTrace(t *testing.T) {
	c, x := purifiedParetoFixture(t)
	negative, err := c.manager.MkArithmeticNeg(x)
	contextMust(t, err)
	contextMust(t, c.AddObjective(negative, true))
	stream := paretoModelVerboseStream(t, c, 1)
	w := &paretoTraceWriter{}
	tags := diagnostic.NewTraceTagsWithWriter(w)
	tags.Enable("opt")
	c.config.ModelEvaluator.Trace = tags
	w.check = func() {
		text, err := stream.String()
		contextMust(t, err)
		if !strings.Contains(text, "new model:\n") {
			t.Fatal("dominance trace preceded model callback")
		}
	}
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	text, err := stream.String()
	contextMust(t, err)
	if !strings.Contains(text, "new model:\n") || strings.Contains(text, "!") {
		t.Fatal("live fixed model callback", text)
	}
	if tags.ShouldTrace("opt") && !strings.Contains(w.String(), "!") {
		t.Fatal("dominance trace lost raw model", w.String())
	}
}

func TestParetoModelDiagnosticAllocationFailuresReleaseCopies(t *testing.T) {
	failures, printedFailure := 0, false
	success := false
	for after := int64(0); after < 256; after++ {
		c, raw, fresh := paretoModelDiagnosticFixture(t)
		stream := paretoModelVerboseStream(t, c, 1)
		refs := raw.References
		c.manager.Budget().FailAfter(after)
		err := c.paretoModelDiagnostic(raw)
		c.manager.Budget().FailAfter(-1)
		if err != nil {
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(after, err)
			}
			failures++
			text, e := stream.String()
			contextMust(t, e)
			printedFailure = printedFailure || strings.HasPrefix(text, "new model:\n")
		} else {
			success = true
		}
		found, e := raw.HasConstantInterpretation(fresh)
		contextMust(t, e)
		if !found || raw.References != refs {
			t.Fatal("failed diagnostic changed raw model", after, found, raw.References, refs)
		}
		if success {
			break
		}
	}
	if !success || failures == 0 || !printedFailure {
		t.Fatal("model/printing frontiers", success, failures, printedFailure)
	}
}
