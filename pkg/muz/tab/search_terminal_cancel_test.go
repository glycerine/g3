package tab

// Source: src/muz/tab/tab_context.cpp imp::run checks cancellation before
// each instruction, including BACKTRACK and both terminal instructions.

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestTabSearchTerminalCancellationAndRetry(t *testing.T) {
	f := newTabDiagnosticsFixture(t)
	m := f.ctx.manager
	session, err := NewSession(m, "weight")
	tabDiagnosticsMust(t, err)
	defer func() { tabDiagnosticsMust(t, session.Close()) }()
	writer := &tabTerminalCancelWriter{manager: m}
	tabCaptureVerbose(t, 2, writer)
	var stats Statistics
	status, answer, err := QueryExpressionWithSessionStatistics(f.rules, f.queries[0], f.rw, f.lite, f.querySolver, f.subSolver, session, &stats)
	m.ResourceLimit().ResetCancel()
	tabDiagnosticsMust(t, err)
	if !writer.fired {
		t.Fatal("native final-clause cancellation callback did not execute")
	}
	if status != lbool.Undef || answer != (ast.ASTRef{}) {
		t.Fatal("canceled terminal instruction published a result", status, answer)
	}
	if stats != (Statistics{Unfold: 2}) || session.nextSequence != 3 || session.selection.next != 18 || len(session.selection.cache) != 2 {
		t.Fatal("terminal cancellation changed completed source work", stats, session.nextSequence, session.selection.next, len(session.selection.cache))
	}
	checkDepth := func() {
		t.Helper()
		for _, backend := range []solver.Solver{f.querySolver, f.subSolver} {
			depth, err := backend.ScopeLevel()
			tabDiagnosticsMust(t, err)
			if depth != 0 {
				t.Fatal("terminal cancellation left a solver scope", depth)
			}
		}
	}
	checkDepth()
	status, answer, err = QueryExpressionWithSessionStatistics(f.rules, f.queries[0], f.rw, f.lite, f.querySolver, f.subSolver, session, &stats)
	tabDiagnosticsMust(t, err)
	if status != lbool.True || stats != (Statistics{Unfold: 4}) || session.nextSequence != 6 || session.selection.next != 16 || len(session.selection.cache) != 2 {
		t.Fatal("terminal cancellation retry changed source search state", status, stats, session.nextSequence, session.selection.next, len(session.selection.cache))
	}
	proof, err := m.RefGet(answer)
	tabDiagnosticsMust(t, err)
	kind, err := m.GetDeclKind(proof)
	tabDiagnosticsMust(t, err)
	fact, err := m.GetProofFact(proof)
	tabDiagnosticsMust(t, err)
	if kind != ast.ASTPrHyperResolve || !m.IsFalse(fact) {
		t.Fatal("retry did not publish a refutation proof")
	}
	tabDiagnosticsMust(t, m.DelRef(&answer))
	checkDepth()
}

type tabTerminalCancelWriter struct {
	manager *ast.ASTManager
	output  strings.Builder
	fired   bool
}

func (w *tabTerminalCancelWriter) Write(body []byte) (int, error) {
	_, _ = w.output.Write(body)
	// The numeric stream may write one character at a time. Cancel only
	// after the final empty-clause line has been completely displayed.
	if !w.fired && strings.HasSuffix(w.output.String(), "true\n") {
		w.fired = true
		w.manager.ResourceLimit().Cancel()
	}
	return len(body), nil
}

func TestTabSearchSourceInstructionCancellationCounts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		empty  bool
		want   lbool.LBool
		checks uint64
	}{
		{"empty-goal", true, lbool.True, 2},
		{"missing-predicate", false, lbool.False, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTabDiagnosticsFixture(t)
			m := f.ctx.manager
			tabCaptureVerbose(t, 0, io.Discard)
			falsehood, err := m.MkFalse()
			tabDiagnosticsMust(t, err)
			truth, err := m.MkTrue()
			tabDiagnosticsMust(t, err)
			var predicates []ast.ASTHandle
			if !tc.empty {
				predicates = []ast.ASTHandle{f.queries[0]}
			}
			goal, err := NewClause(m, f.rw, falsehood, predicates, truth)
			tabDiagnosticsMust(t, err)
			rules := &RuleIndex{manager: m, byHead: make(map[ast.ASTHandle][]int)}
			search, err := newTabSearch(goal, rules, f.rw, f.lite, f.querySolver, f.subSolver, "first")
			if err != nil {
				tabDiagnosticsMust(t, goal.Close())
				t.Fatal(err)
			}
			defer func() { tabDiagnosticsMust(t, search.Close()) }()
			before := m.ResourceLimit().Count()
			status, err := search.Run()
			after := m.ResourceLimit().Count()
			tabDiagnosticsMust(t, err)
			if status != tc.want || after-before != tc.checks {
				t.Fatal("source instruction boundaries", status, tc.want, after-before, tc.checks)
			}
			if search.unfold != 0 || search.unfoldFail != 0 || search.subsumed != 0 || search.nextSequence != 1 || len(search.owned) != 1 {
				t.Fatal("instruction boundaries changed source bookkeeping")
			}
		})
	}
}
