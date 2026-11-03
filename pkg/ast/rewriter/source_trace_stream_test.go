//go:build g3trace

package rewriter

import (
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type rewriterTraceRefusal struct {
	calls, flushes int
	budget         *arena.Budget
}

func (w *rewriterTraceRefusal) Write([]byte) (int, error) {
	w.calls++
	if w.budget != nil {
		w.budget.FailAfter(0)
	}
	return 0, io.ErrClosedPipe
}

func (w *rewriterTraceRefusal) Flush() error {
	w.flushes++
	return io.ErrClosedPipe
}

type rewriterTraceBody func(io.Writer, *uparams.GlobalParamsService) error

func newRewriterTraceOperandFixture(t *testing.T, name string, out io.Writer) (*ast.ASTManager, func(rewriterTraceBody) error) {
	t.Helper()
	tags := diagnostic.NewTraceTagsWithWriter(out)
	m := unusedVarsTraceManager(t, tags)
	bindBVTraceGlobals(t, m)
	var emit func(rewriterTraceBody) error
	switch name {
	case "der":
		r, err := NewDER(m)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		astTestMust(t, r.BindTrace(tags))
		emit = func(body rewriterTraceBody) error { return r.emitDERTrace(name, "operand", 51, body) }
	case "factor_rewriter":
		r, err := NewFactorRewriter(m)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		astTestMust(t, r.BindTrace(tags))
		emit = func(body rewriterTraceBody) error { return r.emitFactorTrace("operand", 166, body) }
	case "ac_sharing_detail":
		r, err := NewMaximizeBVSharingConfiguration(m)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		astTestMust(t, r.BindTrace(tags))
		emit = func(body rewriterTraceBody) error { return r.emitSharingTrace("operand", 1, body) }
	case "push_app_ite":
		r, err := NewPushAppITEConfiguration(m, false)
		astTestMust(t, err)
		astTestMust(t, r.BindTrace(tags))
		emit = func(body rewriterTraceBody) error { return r.emitPushITETrace("operand", 51, body) }
	case "elim_unused_vars":
		emit = func(body rewriterTraceBody) error { return emitUnusedVarsTrace(m, "operand", 98, body) }
	default:
		t.Fatal("unknown trace fixture", name)
	}
	tags.Enable(name)
	return m, emit
}

func TestRewriterTraceStreamContinuesOperandsAndPreservesTheirErrors(t *testing.T) {
	for _, name := range []string{"der", "factor_rewriter", "ac_sharing_detail", "push_app_ite", "elim_unused_vars"} {
		t.Run(name, func(t *testing.T) {
			out := &rewriterTraceRefusal{}
			_, emit := newRewriterTraceOperandFixture(t, name, out)
			visits := 0
			for record := 0; record < 2; record++ {
				astTestMust(t, emit(func(w io.Writer, _ *uparams.GlobalParamsService) error {
					if _, err := io.WriteString(w, "before operand"); err != nil {
						return err
					}
					visits++
					_, err := w.Write([]byte("after operand"))
					return err
				}))
			}
			if visits != 2 || out.calls != 1 || out.flushes != 2 {
				t.Fatal("bad stream changed reached operands or physical output", visits, out.calls, out.flushes)
			}
			// An operand may fail with the same error value as the writer. It
			// still must escape instead of being mistaken for stream badbit.
			for _, cause := range []error{ast.ErrASTHandle, io.ErrClosedPipe} {
				reached := false
				err := emit(func(w io.Writer, _ *uparams.GlobalParamsService) error {
					if _, err := io.WriteString(w, "before failing operand"); err != nil {
						return err
					}
					reached = true
					return cause
				})
				if !reached || !errors.Is(err, cause) || out.calls != 1 || out.flushes != 2 {
					t.Fatal("trace operand failure changed", reached, err, cause, out.calls, out.flushes)
				}
			}
		})
	}
}

func TestRewriterTraceBadStreamStillComputesFormattingOperand(t *testing.T) {
	for _, name := range []string{"der", "factor_rewriter", "ac_sharing_detail", "push_app_ite", "elim_unused_vars"} {
		t.Run(name, func(t *testing.T) {
			out := &rewriterTraceRefusal{}
			m, emit := newRewriterTraceOperandFixture(t, name, out)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			before := m.Stats().LiveReferences
			out.budget = m.Budget()
			reached := false
			err = emit(func(w io.Writer, globals *uparams.GlobalParamsService) error {
				if _, err := io.WriteString(w, "before formatted operand"); err != nil {
					return err
				}
				reached = true
				return writeBVTraceSMT2(m, w, truth, globals)
			})
			m.Budget().FailAfter(-1)
			if !reached || !errors.Is(err, arena.ErrInjectedFailure) || out.calls != 1 || out.flushes != 0 {
				t.Fatal("bad stream suppressed formatting failure", reached, err, out.calls, out.flushes)
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("failed trace operand changed reference ownership", got, before)
			}
		})
	}
}
