package smtcore

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type optimizationCheckTraceWriter struct {
	bytes.Buffer
	check func()
	fail  bool
}

func (w *optimizationCheckTraceWriter) Write(p []byte) (int, error) {
	if w.check != nil {
		w.check()
	}
	if w.fail {
		return 0, io.ErrClosedPipe
	}
	return w.Buffer.Write(p)
}

func optimizationCheckTraceFixture(t *testing.T) (*SMTSolver, *diagnostic.TraceTags, *optimizationCheckTraceWriter) {
	t.Helper()
	m, s := optimizationSMTFixture(t, true)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	for _, name := range []string{"trace-first", "trace-second"} {
		p, err := m.MkConst(astTestSymbol(t, m, name), boolean)
		astTestMust(t, err)
		astTestMust(t, s.Assert(p))
	}
	w := &optimizationCheckTraceWriter{}
	tags := diagnostic.NewTraceTagsWithWriter(w)
	tags.Enable("opt_verbose")
	s.config.ModelEvaluator.Trace = tags
	return s, tags, w
}

func TestOptimizationSolverContextTraceBeforeDumpAndSearch(t *testing.T) {
	optimizationDumpTestDirectory(t)
	s, tags, w := optimizationCheckTraceFixture(t)
	if !tags.ShouldTrace("opt_verbose") {
		t.Skip("source TRACE erased")
	}
	optimizationDumpParameters(t, s, true)
	w.check = func() {
		if optimizationDumpCount != 0 || s.kernel.internalizer.statistics.NumChecks != 0 {
			t.Fatal("trace followed benchmark/search")
		}
		if _, err := os.Stat("opt_solver1.smt2"); !os.IsNotExist(err) {
			t.Fatal("trace followed file creation", err)
		}
	}
	boolean, err := s.manager.MkBoolSort()
	astTestMust(t, err)
	assumption, err := s.manager.MkConst(astTestSymbol(t, s.manager, "excluded-assumption"), boolean)
	astTestMust(t, err)
	astTestPin(t, s.manager, assumption)
	status, err := s.CheckSat([]ast.ASTHandle{assumption})
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	text := w.String()
	if !strings.Contains(text, "src/opt/opt_solver.cpp:162") || !strings.Contains(text, "context size: 2\ntrace-first\ntrace-second\n") || strings.Contains(text, "excluded-assumption") {
		t.Fatal("source context trace", text)
	}
	if _, err = os.Stat("opt_solver1.smt2"); err != nil {
		t.Fatal("trace prevented later benchmark", err)
	}
}

func TestOptimizationSolverContextTraceFailureStillEvaluatesOperands(t *testing.T) {
	for _, operandFails := range []bool{false, true} {
		optimizationDumpTestDirectory(t)
		s, tags, w := optimizationCheckTraceFixture(t)
		if !tags.ShouldTrace("opt_verbose") {
			t.Skip("source TRACE erased")
		}
		w.fail = true
		if operandFails {
			w.check = func() { s.manager.Budget().FailAfter(0) }
		}
		status, err := s.CheckSat(nil)
		s.manager.Budget().FailAfter(-1)
		if operandFails {
			if !errors.Is(err, arena.ErrInjectedFailure) || s.kernel.internalizer.statistics.NumChecks != 0 || !s.optimization.first {
				t.Fatal("trace operand failure did not stop before check", status, err)
			}
		} else {
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("stream refusal changed check", status)
			}
		}
	}
}

func TestOptimizationSolverContextTraceErasesDisabledAndOrdinaryOperands(t *testing.T) {
	for _, optimize := range []bool{false, true} {
		m, s := optimizationSMTFixture(t, optimize)
		w := &optimizationCheckTraceWriter{}
		tags := diagnostic.NewTraceTagsWithWriter(w)
		if !optimize {
			tags.Enable("opt_verbose")
		}
		s.config.ModelEvaluator.Trace = tags
		m.Budget().FailAfter(0)
		err := s.traceOptimizationCheck()
		m.Budget().FailAfter(-1)
		astTestMust(t, err)
		if w.Len() != 0 {
			t.Fatal("erased trace wrote output")
		}
	}
}

func TestOptimizationSolverContextTraceReleasesFormatDocuments(t *testing.T) {
	s, tags, _ := optimizationCheckTraceFixture(t)
	if !tags.ShouldTrace("opt_verbose") {
		t.Skip("source TRACE erased")
	}
	astTestMust(t, s.traceOptimizationCheck())
	formats, err := s.manager.GetFormatManager()
	astTestMust(t, err)
	before := formats.Stats()
	boolean, err := s.manager.MkBoolSort()
	astTestMust(t, err)
	for _, name := range []string{"fresh-trace-one", "fresh-trace-two", "fresh-trace-three"} {
		p, err := s.manager.MkConst(astTestSymbol(t, s.manager, name), boolean)
		astTestMust(t, err)
		astTestMust(t, s.Assert(p))
		astTestMust(t, s.traceOptimizationCheck())
		after := formats.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
			t.Fatal("trace kept temporary format trees", before, after)
		}
	}
}

func TestOptimizationSolverTraceExpressionUsesGlobalPrinterProfile(t *testing.T) {
	m, s := optimizationSMTFixture(t, true)
	literal, err := m.BitVectorNumeralUnsigned(15, 4)
	astTestMust(t, err)
	astTestPin(t, m, literal)
	for _, test := range []struct{ flag, want string }{{"true", "#xf"}, {"false", "(_ bv15 4)"}} {
		astTestMust(t, s.kernel.paramGlobals.SourceSetName("pp.bv_literals", test.flag))
		var out bytes.Buffer
		astTestMust(t, s.writeOptimizationExpression(&out, literal))
		if out.String() != test.want {
			t.Fatal("global expression profile", test.flag, out.String(), test.want)
		}
	}
}
