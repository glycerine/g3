package sat

// Source: src/sat/sat_solver.cpp:1191-1196,1347-1355.

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestSATSolverSimplifyAndRestartSourceVerboseLevels(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	out := satInspectionStream(t, arena.NewBudget(8192))
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	satOK(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	satOK(t, err)
	t.Cleanup(func() {
		satOK(t, diagnostic.SetProcessVerboseStream(oldStream))
		satOK(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
	})
	satOK(t, diagnostic.SetProcessVerboseStream(out))
	satOK(t, diagnostic.SetProcessVerbosityLevel(2))
	satOK(t, s.simplifyProblem())
	got, err := out.String()
	satOK(t, err)
	if got != "(sat.simplify)\n" {
		t.Fatalf("source simplify notice: %q", got)
	}
	satOK(t, out.ResetText())
	s.nextSimplify = s.conflict.conflicts + 1
	satOK(t, s.simplifyProblem())
	got, err = out.String()
	satOK(t, err)
	if got != "" {
		t.Fatalf("early simplify exit emitted notice: %q", got)
	}
	satOK(t, diagnostic.SetProcessVerbosityLevel(1))
	satOK(t, s.gc.initSearch())
	s.conflict.conflicts = 7
	s.decision.decisions = 3
	satOK(t, s.gc.restart())
	got, err = out.String()
	satOK(t, err)
	want := regexp.MustCompile(`^\(sat-restart :conflicts 7 :decisions 3 :restarts 1 :gc-clause 0 :memory [0-9]+\.[0-9]{2} :time [0-9]+\.[0-9]{2}\)\n$`)
	if !want.MatchString(got) || !s.gc.watch.Running() {
		t.Fatalf("source restart report and running stopwatch: %q", got)
	}
	satOK(t, out.ResetText())
	satOK(t, diagnostic.SetProcessVerbosityLevel(30))
	satOK(t, s.gc.restart())
	got, err = out.String()
	satOK(t, err)
	want = regexp.MustCompile(`^\(sat-restart :conflicts 7 :decisions 3 :restarts 2 :gc-clause 0 :memory [0-9]+\.[0-9]{2} :time [0-9]+\.[0-9]{2}\)\n`)
	if !want.MatchString(got) || !strings.Contains(got, "(sat-status\n  :inconsistent    false\n") {
		t.Fatalf("source level-thirty status after restart: %q", got)
	}
}

func TestSATSolverCheckVerboseFileFailureKeepsSearchResult(t *testing.T) {
	s, c, p := satSolverCheckFixture(t)
	reader, writer, err := os.Pipe()
	satOK(t, err)
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: arena.NewBudget(8192), BorrowedFile: writer})
	satOK(t, err)
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	satOK(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	satOK(t, err)
	t.Cleanup(func() {
		satOK(t, diagnostic.SetProcessVerboseStream(oldStream))
		satOK(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		satOK(t, out.Close())
		_ = writer.Close()
		_ = reader.Close()
	})
	satOK(t, diagnostic.SetProcessVerboseStream(out))
	satOK(t, diagnostic.SetProcessVerbosityLevel(10))
	satOK(t, reader.Close())
	k := "max_conflicts"
	satOK(t, c.SetUIntName(&p, &k, 0))
	satOK(t, s.UpdateParams(&p))
	result, err := s.Check(nil)
	satOK(t, err)
	if result != lbool.Undef || !out.FileBad {
		t.Fatal("failed verbose file changed source search result or missed sticky badbit", result, out.FileBad)
	}
}

func TestSATSolverCheckSourceRestartLimitAbort(t *testing.T) {
	s, c, p := satSolverCheckFixture(t)
	out := satInspectionStream(t, arena.NewBudget(8192))
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	satOK(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	satOK(t, err)
	t.Cleanup(func() {
		satOK(t, diagnostic.SetProcessVerboseStream(oldStream))
		satOK(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
	})
	satOK(t, diagnostic.SetProcessVerboseStream(out))
	satOK(t, diagnostic.SetProcessVerbosityLevel(10))
	for _, setting := range []struct {
		name  string
		value uint32
	}{
		{"burst_search", 0},
		{"restart.initial", 0},
		{"restart.max", 0},
		{"max_conflicts", 100},
	} {
		satOK(t, c.SetUIntName(&p, &setting.name, setting.value))
	}
	satOK(t, s.UpdateParams(&p))
	// A prior simplify horizon lets search reach its restart boundary.
	s.nextSimplify = ^uint32(0)
	for i := 0; i < 2; i++ {
		_, err = s.MkVar(false, true)
		satOK(t, err)
	}
	for i := 0; i < 4; i++ {
		satOK(t, s.MkClause([]Literal{MakeLiteral(0, i&1 != 0), MakeLiteral(1, i&2 != 0)}))
	}
	result, err := s.Check(nil)
	satOK(t, err)
	got, err := out.String()
	satOK(t, err)
	if result != lbool.Undef || !strings.Contains(got, "(sat \"abort: max-restarts\")\n") {
		t.Fatal("source restart-limit exit", result, got)
	}
}

func TestSATSolverCheckReleaseOmitsModelNotice(t *testing.T) {
	for _, level := range []uint32{0, 10} {
		t.Run(strconv.FormatUint(uint64(level), 10), func(t *testing.T) {
			s, _, _ := satSolverCheckFixture(t)
			out := satInspectionStream(t, arena.NewBudget(8192))
			oldLevel, err := diagnostic.ProcessVerbosityLevel()
			satOK(t, err)
			oldStream, err := diagnostic.BorrowProcessVerboseStream()
			satOK(t, err)
			t.Cleanup(func() {
				satOK(t, diagnostic.SetProcessVerboseStream(oldStream))
				satOK(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
			})
			satOK(t, diagnostic.SetProcessVerboseStream(out))
			satOK(t, diagnostic.SetProcessVerbosityLevel(level))
			result, err := s.Check(nil)
			satOK(t, err)
			got, err := out.String()
			satOK(t, err)
			if result != lbool.True {
				t.Fatal("empty formula should produce a checked model", result)
			}
			if level == 0 && got != "" {
				t.Fatal("disabled verbosity emitted model notice", got)
			}
			// AUDIT.md FUZZ-F7: the "checking model" line is #ifndef
			// _EXTERNAL_RELEASE; g3 follows the release build.
			if level == 10 {
				if strings.Index(got, "(sat.sat-solver)\n") != 0 || strings.Contains(got, "checking model") {
					t.Fatal("release-profile check notice", got)
				}
			}
		})
	}
}

func TestSATSolverCheckSourceEntryAndZeroConflictAbortVerbosity(t *testing.T) {
	for _, level := range []uint32{0, 2, 10} {
		t.Run(strconv.FormatUint(uint64(level), 10), func(t *testing.T) {
			s, c, p := satSolverCheckFixture(t)
			out := satInspectionStream(t, arena.NewBudget(8192))
			oldLevel, err := diagnostic.ProcessVerbosityLevel()
			satOK(t, err)
			oldStream, err := diagnostic.BorrowProcessVerboseStream()
			satOK(t, err)
			t.Cleanup(func() {
				satOK(t, diagnostic.SetProcessVerboseStream(oldStream))
				satOK(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
			})
			satOK(t, diagnostic.SetProcessVerboseStream(out))
			satOK(t, diagnostic.SetProcessVerbosityLevel(level))
			k := "max_conflicts"
			satOK(t, c.SetUIntName(&p, &k, 0))
			satOK(t, s.UpdateParams(&p))
			result, err := s.Check(nil)
			satOK(t, err)
			if result != lbool.Undef {
				t.Fatal("zero-conflict source return", result)
			}
			got, err := out.String()
			satOK(t, err)
			if level == 0 && got != "" {
				t.Fatal("disabled verbosity emitted output", got)
			}
			if level >= 2 && !strings.HasPrefix(got, "(sat.sat-solver)\n") {
				t.Fatal("check entry missing before search", got)
			}
			abort := "(sat \"abort: max-conflicts = 0\")\n"
			if (level >= 10) != strings.Contains(got, abort) {
				t.Fatal("source abort gate or text", got)
			}
		})
	}
}
