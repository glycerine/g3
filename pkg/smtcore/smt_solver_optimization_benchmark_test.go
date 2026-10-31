package smtcore

import (
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/stopwatch"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func optimizationDumpTestDirectory(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	previous := optimizationDumpCount
	optimizationDumpCount = 0
	t.Cleanup(func() { optimizationDumpCount = previous })
}

func optimizationDumpParameters(t *testing.T, s *SMTSolver, enabled bool) {
	t.Helper()
	p, err := s.config.Params.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, s.config.Params.Del(&p)) }()
	key := "dump_benchmarks"
	astTestMust(t, s.config.Params.SetBoolName(&p, &key, enabled))
	astTestMust(t, s.UpdateParams(&p))
}

func optimizationDumpVerboseStream(t *testing.T, s *SMTSolver) *utext.NumericTextStream {
	t.Helper()
	old, err := diagnostic.BorrowProcessVerboseStream()
	astTestMust(t, err)
	level, err := diagnostic.ProcessVerbosityLevel()
	astTestMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: s.manager.Budget()})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, diagnostic.SetProcessVerboseStream(old))
		astTestMust(t, diagnostic.SetProcessVerbosityLevel(level))
		astTestMust(t, stream.Close())
	})
	astTestMust(t, diagnostic.SetProcessVerboseStream(stream))
	astTestMust(t, diagnostic.SetProcessVerbosityLevel(1))
	return stream
}

func TestOptimizationSolverBenchmarkDumpOrderNumberingAndVerbosity(t *testing.T) {
	optimizationDumpTestDirectory(t)
	m, s := optimizationSMTFixture(t, true)
	stream := optimizationDumpVerboseStream(t, s)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "dump-a"), boolean)
	astTestMust(t, err)
	astTestPin(t, m, a)
	q, err := m.MkConst(astTestSymbol(t, m, "dump-q"), boolean)
	astTestMust(t, err)
	astTestMust(t, s.Assert(q))
	optimizationDumpParameters(t, s, true)
	status, err := s.CheckSat([]ast.ASTHandle{a})
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	body, err := os.ReadFile("opt_solver1.smt2")
	astTestMust(t, err)
	want := "; opt_solver\n(set-info :status unknown)\n(declare-fun dump-a () Bool)\n(declare-fun dump-q () Bool)\n(assert\n dump-a)\n(assert\n dump-q)\n(check-sat)\n"
	if string(body) != want {
		t.Fatalf("source benchmark ordering\n%s\nwant\n%s", body, want)
	}
	text, err := stream.String()
	astTestMust(t, err)
	if !regexp.MustCompile(`\(created benchmark: opt_solver1\.smt2\.{5} l_true [0-9]+\.[0-9]{6}\)\n`).MatchString(text) {
		t.Fatal("source verbose timing", text)
	}
	if stream.State().FloatMode != utext.NumericFixedFloat {
		t.Fatal("fixed formatting did not persist")
	}
	// The counter belongs to opt_solver, not an individual solver instance.
	_, other := optimizationSMTFixture(t, true)
	optimizationDumpParameters(t, other, true)
	status, err = other.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	if _, err = os.Stat("opt_solver2.smt2"); err != nil {
		t.Fatal(err)
	}
	optimizationDumpParameters(t, s, false)
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || optimizationDumpCount != 2 {
		t.Fatal("disabled dump", status, optimizationDumpCount)
	}
}

func TestOptimizationSolverBenchmarkConstructorStartsDisabled(t *testing.T) {
	optimizationDumpTestDirectory(t)
	m, cfg := smtModelGeneratorFixture(t)
	p, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&p)) })
	key := "dump_benchmarks"
	astTestMust(t, cfg.Context.SetBoolName(&p, &key, true))
	s, err := NewOptimizationSMTSolver(m, &p, "AUFLIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || s.optimization.dumpBenchmarks || optimizationDumpCount != 0 {
		t.Fatal("constructor read update-only dump flag", status, optimizationDumpCount)
	}
	files, err := os.ReadDir(".")
	astTestMust(t, err)
	if len(files) != 0 {
		t.Fatal("disabled constructor wrote files", files)
	}
	astTestMust(t, s.UpdateParams(&p))
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || optimizationDumpCount != 1 {
		t.Fatal("explicit update did not enable dump")
	}
}

func TestOptimizationSolverBenchmarkFileFailureStillRunsPrinter(t *testing.T) {
	optimizationDumpTestDirectory(t)
	_, s := optimizationSMTFixture(t, true)
	optimizationDumpParameters(t, s, true)
	stream := optimizationDumpVerboseStream(t, s)
	stream.FileBad = true
	astTestMust(t, os.Mkdir("opt_solver1.smt2", 0700))
	serial := s.optimization.parameters.Serial
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || optimizationDumpCount != 1 || s.optimization.parameters.Serial != serial+1 || s.optimization.parameters.Helpers != nil {
		t.Fatal("failed file suppressed printer operands", status, optimizationDumpCount, s.optimization.parameters.Serial)
	}
	if stream.State().FloatMode != utext.NumericFixedFloat {
		t.Fatal("bad stream suppressed fixed manipulator")
	}
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	body, err := os.ReadFile("opt_solver2.smt2")
	astTestMust(t, err)
	if status != lbool.True || !strings.HasSuffix(string(body), "(check-sat)\n") {
		t.Fatal("failed file broke subsequent checks", status, string(body))
	}
}

func TestOptimizationSolverBenchmarkGlobalFallbackAndLocalOverride(t *testing.T) {
	optimizationDumpTestDirectory(t)
	_, s := optimizationSMTFixture(t, true)
	astTestMust(t, s.kernel.paramGlobals.SourceSetName("opt.dump_benchmarks", "true"))
	astTestMust(t, s.UpdateParams(nil))
	if !s.optimization.dumpBenchmarks {
		t.Fatal("global opt flag ignored")
	}
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || optimizationDumpCount != 1 {
		t.Fatal("global dump did not execute", status, optimizationDumpCount)
	}
	optimizationDumpParameters(t, s, false)
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || optimizationDumpCount != 1 {
		t.Fatal("local false did not override global true")
	}
}

func TestOptimizationSolverBenchmarkUsesGlobalPrettyPrinterDefaults(t *testing.T) {
	m, s := optimizationSMTFixture(t, true)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	var args []ast.ASTHandle
	for _, name := range []string{"dump-a", "dump-b", "dump-c"} {
		arg, err := m.MkConst(astTestSymbol(t, m, name), boolean)
		astTestMust(t, err)
		astTestPin(t, m, arg)
		args = append(args, arg)
	}
	inner, err := m.MkImplies(args[1], args[2])
	astTestMust(t, err)
	outer, err := m.MkImplies(args[0], inner)
	astTestMust(t, err)
	astTestPin(t, m, outer)
	for _, test := range []struct{ flag, want string }{
		{"false", "(=> dump-a (=> dump-b dump-c))"},
		{"true", "(=> (and dump-a dump-b) dump-c)"},
	} {
		astTestMust(t, s.kernel.paramGlobals.SourceSetName("pp.simplify_implies", test.flag))
		var out strings.Builder
		astTestMust(t, s.writeOptimizationBenchmark(&out, []ast.ASTHandle{outer}))
		if !strings.Contains(out.String(), test.want) {
			t.Fatal("pp defaults not applied", test.flag, out.String())
		}
		if s.optimization.parameters.Helpers != nil {
			t.Fatal("printer parameter helper survived return")
		}
	}
}

func TestOptimizationSolverBenchmarkPreparationFailureFrontier(t *testing.T) {
	optimizationDumpTestDirectory(t)
	failures, success := 0, false
	for after := int64(0); after < 128; after++ {
		m, s := optimizationSMTFixture(t, true)
		optimizationDumpParameters(t, s, true)
		before := optimizationDumpCount
		watch := stopwatch.New(nil)
		m.Budget().FailAfter(after)
		err := s.beginOptimizationBenchmark(&watch, nil)
		m.Budget().FailAfter(-1)
		if optimizationDumpCount != before+1 || !s.optimization.first || s.kernel.internalizer.statistics.NumChecks != 0 {
			t.Fatal("pre-check dump frontier order", after, optimizationDumpCount, before)
		}
		if err != nil {
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(after, err)
			}
			failures++
		} else {
			success = true
		}
		// Incomplete generated-parameter constructors remain with the concrete
		// solver owner until this explicit, retryable cleanup.
		astTestMust(t, s.Close())
		if success {
			break
		}
	}
	if failures == 0 || !success {
		t.Fatal("dump preparation frontier", failures, success)
	}
}

type optimizationRefusingBenchmarkFile struct {
	writes int
	short  bool
}

func (w *optimizationRefusingBenchmarkFile) Write([]byte) (int, error) {
	w.writes++
	if w.short {
		return 0, nil
	}
	return 0, io.ErrClosedPipe
}

func TestOptimizationSolverBenchmarkSuppressesFileErrorsAfterFirstRefusal(t *testing.T) {
	for _, short := range []bool{false, true} {
		_, s := optimizationSMTFixture(t, true)
		file := &optimizationRefusingBenchmarkFile{short: short}
		out := &optimizationBenchmarkWriter{out: file}
		astTestMust(t, s.writeOptimizationBenchmark(out, nil))
		if file.writes != 1 || !out.failed || s.optimization.parameters.Helpers != nil {
			t.Fatal("file badbit/operand completion", file.writes, out.failed)
		}
	}
}
