package tab

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestTabExpressionPreservesNegativeQueryGuard(t *testing.T) {
	f := newTabDiagnosticsFixture(t)
	query, err := f.ctx.manager.MkNot(f.queries[0])
	tabDiagnosticsMust(t, err)
	session, err := NewSession(f.ctx.manager, "weight")
	tabDiagnosticsMust(t, err)
	defer func() { tabDiagnosticsMust(t, session.Close()) }()
	status, answer, err := QueryExpressionWithSession(f.rules, query, f.rw, f.lite, f.querySolver, f.subSolver, session)
	var invalid *base.RulePropertyError
	if status != lbool.Undef || answer != (ast.ASTRef{}) || !errors.As(err, &invalid) || invalid.Kind != base.RulePropertyNegativePredicate {
		t.Fatal("negative query escaped the fragment guard", status, answer, err)
	}
	if session.nextSequence != 0 || len(session.selection.cache) != 0 {
		t.Fatal("rejected query entered source search state")
	}
}

func TestTabExpressionConstantAndTheoryGoalsNeedNoProgramUnfold(t *testing.T) {
	f := newTabDiagnosticsFixture(t)
	m := f.ctx.manager
	session, err := NewSession(m, "weight")
	tabDiagnosticsMust(t, err)
	defer func() { tabDiagnosticsMust(t, session.Close()) }()
	truth, err := m.MkTrue()
	tabDiagnosticsMust(t, err)
	falsehood, err := m.MkFalse()
	tabDiagnosticsMust(t, err)
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		tabDiagnosticsMust(t, err)
		ref, err := m.NewRef(h)
		tabDiagnosticsMust(t, err)
		t.Cleanup(func() { tabDiagnosticsMust(t, m.DelRef(&ref)) })
		return h
	}
	zero := own(m.ArithmeticIntegerNumeralText("0"))
	one := own(m.ArithmeticIntegerNumeralText("1"))
	integer, err := m.GetSort(zero)
	tabDiagnosticsMust(t, err)
	x := own(m.MkVar(0, integer))
	xZero := own(m.MkEq(x, zero))
	xOne := own(m.MkEq(x, one))
	impossible := own(m.MkAnd(xZero, xOne))
	for _, tc := range []struct {
		query ast.ASTHandle
		want  lbool.LBool
	}{{truth, lbool.True}, {falsehood, lbool.False}, {xZero, lbool.True}, {impossible, lbool.False}} {
		var stats Statistics
		status, answer, err := QueryExpressionWithSessionStatistics(f.rules, tc.query, f.rw, f.lite, f.querySolver, f.subSolver, session, &stats)
		tabDiagnosticsMust(t, err)
		if status != tc.want || stats != (Statistics{}) || len(session.selection.cache) != 0 {
			t.Fatal("constant goal entered program search", status, tc.want, stats)
		}
		handle, err := m.RefGet(answer)
		tabDiagnosticsMust(t, err)
		if status == lbool.True {
			fact, err := m.GetProofFact(handle)
			tabDiagnosticsMust(t, err)
			if !m.IsFalse(fact) {
				t.Fatal("theory-only query did not produce a refutation")
			}
		} else if !m.IsTrue(handle) {
			t.Fatal("unreachable theory-only answer is not true")
		}
		tabDiagnosticsMust(t, m.DelRef(&answer))
	}
}

func TestTabExpressionUsesSourceInitialGoalAndPersistentSelection(t *testing.T) {
	f := newTabDiagnosticsFixture(t)
	m := f.ctx.manager
	session, err := NewSession(m, "weight")
	tabDiagnosticsMust(t, err)
	defer func() { tabDiagnosticsMust(t, session.Close()) }()
	var stats Statistics
	for attempt := uint32(1); attempt <= 2; attempt++ {
		status, answer, err := QueryExpressionWithSessionStatistics(f.rules, f.queries[0], f.rw, f.lite, f.querySolver, f.subSolver, session, &stats)
		tabDiagnosticsMust(t, err)
		if status != lbool.True || stats != (Statistics{Unfold: 2 * attempt}) {
			t.Fatal("expression added a synthetic unfold", status, stats)
		}
		if session.nextSequence != 3*attempt || session.selection.next != 20-2*attempt || session.selection.multiplier != 1 {
			t.Fatal("expression changed source sequence or selection decay", session.nextSequence, session.selection.next, session.selection.multiplier)
		}
		// The source selects only P(0) and Q(0), including on repeated queries.
		if len(session.selection.cache) != 2 || len(session.selection.refs) != 2 {
			t.Fatal("expression cached an additional query predicate", len(session.selection.cache), len(session.selection.refs))
		}
		if _, found := session.selection.cache[f.queries[0]]; !found {
			t.Fatal("source initial predicate was not selected")
		}
		proof, err := m.RefGet(answer)
		tabDiagnosticsMust(t, err)
		fact, err := m.GetProofFact(proof)
		tabDiagnosticsMust(t, err)
		if !m.IsFalse(fact) {
			t.Fatal("expression proof conclusion is not false")
		}
		hyper, asserted := 0, 0
		seen := make(map[ast.ASTHandle]bool)
		pending := []ast.ASTHandle{proof}
		for len(pending) != 0 {
			current := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if seen[current] {
				continue
			}
			seen[current] = true
			kind, err := m.GetDeclKind(current)
			tabDiagnosticsMust(t, err)
			if kind == ast.ASTPrHyperResolve {
				hyper++
			} else if kind == ast.ASTPrAsserted {
				asserted++
			}
			count, err := m.GetProofNumParents(current)
			tabDiagnosticsMust(t, err)
			for i := uint32(0); i < count; i++ {
				parent, err := m.GetProofParent(current, i)
				tabDiagnosticsMust(t, err)
				pending = append(pending, parent)
			}
		}
		if hyper != 2 || asserted != 3 {
			t.Fatal("proof contains a synthetic query bridge", hyper, asserted)
		}
		tabDiagnosticsMust(t, m.DelRef(&answer))
	}
	for _, backend := range []solver.Solver{f.querySolver, f.subSolver} {
		depth, err := backend.ScopeLevel()
		tabDiagnosticsMust(t, err)
		if depth != 0 {
			t.Fatal("expression left solver scopes", depth)
		}
	}
	count, err := f.rules.Len()
	tabDiagnosticsMust(t, err)
	if count != 2 {
		t.Fatal("expression changed the original program", count)
	}
}

func TestTabExpressionCancellationReleasesGoalAndCanRetry(t *testing.T) {
	f := newTabDiagnosticsFixture(t)
	m := f.ctx.manager
	session, err := NewSession(m, "weight")
	tabDiagnosticsMust(t, err)
	defer func() { tabDiagnosticsMust(t, session.Close()) }()
	var stats Statistics
	// Cancel through the existing verbose stream after query-rule
	// normalization and initial-clause ownership have completed.
	writer := &tabExpressionCancelWriter{manager: m}
	tabCaptureVerbose(t, 1, writer)
	status, answer, err := QueryExpressionWithSessionStatistics(f.rules, f.queries[0], f.rw, f.lite, f.querySolver, f.subSolver, session, &stats)
	m.ResourceLimit().ResetCancel()
	tabDiagnosticsMust(t, err)
	if status != lbool.Undef || answer != (ast.ASTRef{}) || stats != (Statistics{}) || len(session.selection.cache) != 0 || session.nextSequence != 1 {
		t.Fatal("canceled expression published search work", status, answer, stats, session.nextSequence)
	}
	status, answer, err = QueryExpressionWithSessionStatistics(f.rules, f.queries[0], f.rw, f.lite, f.querySolver, f.subSolver, session, &stats)
	tabDiagnosticsMust(t, err)
	if status != lbool.True || stats != (Statistics{Unfold: 2}) || session.nextSequence != 4 || session.selection.next != 18 || len(session.selection.cache) != 2 {
		t.Fatal("expression retry changed source search state", status, stats, session.nextSequence, session.selection.next)
	}
	tabDiagnosticsMust(t, m.DelRef(&answer))
}

type tabExpressionCancelWriter struct {
	manager *ast.ASTManager
	fired   bool
}

func (w *tabExpressionCancelWriter) Write(body []byte) (int, error) {
	if !w.fired {
		w.fired = true
		w.manager.ResourceLimit().Cancel()
	}
	return len(body), nil
}
