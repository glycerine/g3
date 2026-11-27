package smtcore

import (
	"fmt"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTScopedClauseUsesOnlyCommittedUniversals(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{
		Params: cfg.Context, ModelEvaluator: cfg, ReduceScopedClauses: true,
	})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	// Keep the source rows in their original form while testing the handoff
	// to search, independently of macro discovery and other rewrite passes.
	s.kernel.preprocess = false
	astTestMust(t, s.SetProduceModels(true))
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "clause-U"), nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	pred := smtQuickPredicate(t, m, "clause-p", sort, boolean)
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	body, err := m.MkApp(pred, []ast.ASTHandle{x})
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort},
		Names: []symbol.Symbol{astTestSymbol(t, m, "x")}, Body: body, Weight: 1})
	astTestMust(t, err)
	free := smtRelevancyBool(t, m, "remaining-literal")
	negatives := make([]ast.ASTHandle, minScopedClauseWidth-1)
	var first ast.ASTHandle
	for i := range negatives {
		a, err := m.MkConst(astTestSymbol(t, m, fmt.Sprintf("clause-a%d", i)), sort)
		astTestMust(t, err)
		app, err := m.MkApp(pred, []ast.ASTHandle{a})
		astTestMust(t, err)
		if i == 0 {
			first = app
		}
		negatives[i], err = m.MkNot(app)
		astTestMust(t, err)
	}
	clause, err := m.MkOrArray(append(negatives, free))
	astTestMust(t, err)
	held, err := m.NewRef(clause)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&held)) }()
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(q))
	if _, changed, err := s.kernel.remainingClauseLiterals(clause); err != nil || changed {
		t.Fatal("used a pending premise", changed, err)
	}
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(clause))
	remaining, changed, err := s.kernel.remainingClauseLiterals(clause)
	astTestMust(t, err)
	if !ast.ASTSourceDebug && !diagnostic.TraceCompiled() && (!changed || !slices.Equal(remaining, []ast.ASTHandle{free})) {
		t.Fatal("expected a single surviving literal", remaining)
	}
	// Ordinary assertions and the proof-producing path must both retain
	// their original inputs when the optimization has to decline.
	m.Budget().FailAfter(0)
	_, changed, err = s.kernel.remainingClauseLiterals(clause)
	m.Budget().FailAfter(-1)
	if err != nil || changed {
		t.Fatal("injected allocation failure changed the clause", changed, err)
	}
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	_, changed, err = s.kernel.remainingClauseLiterals(clause)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	if err != nil || changed {
		t.Fatal("proof-producing clause was reduced", changed, err)
	}
	m.ResourceLimit().IncCancel()
	_, changed, err = s.kernel.remainingClauseLiterals(clause)
	m.ResourceLimit().DecCancel()
	if err != nil || changed {
		t.Fatal("cancelled reduction", changed, err)
	}

	astTestMust(t, s.kernel.internalizePending())
	if _, present := s.kernel.internalizer.BoolVar(first); present && !ast.ASTSourceDebug && !diagnostic.TraceCompiled() {
		t.Fatal("internalized a literal already refuted by the committed premise")
	}
	last, err := s.Assertion(uint32(len(s.kernel.assertions) - 1))
	astTestMust(t, err)
	if last != clause {
		t.Fatal("public assertion changed", last)
	}
	check := func(want lbool.LBool, assumptions ...ast.ASTHandle) {
		t.Helper()
		status, err := s.CheckSat(assumptions)
		if err != nil || status != want {
			t.Fatalf("got %v, want %v, error %v", status, want, err)
		}
	}
	check(lbool.True)
	md, err := s.AcquireModel()
	astTestMust(t, err)
	ev, err := model.NewModelEvaluator(md, cfg)
	astTestMust(t, err)
	astTestMust(t, ev.SetModelCompletion(true))
	value, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	for _, h := range []ast.ASTHandle{clause, first, free} {
		astTestMust(t, ev.Evaluate(h, value))
		v, err := m.RefGet(value)
		astTestMust(t, err)
		if !m.IsTrue(v) {
			t.Fatal("model violates the original assertion or a required literal", h)
		}
	}
	astTestMust(t, m.DelRef(&value))
	astTestMust(t, ev.Close())
	astTestMust(t, md.DecRef())
	notFree, err := m.MkNot(free)
	astTestMust(t, err)
	heldNot, err := m.NewRef(notFree)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&heldNot)) }()
	check(lbool.False, notFree)
	core, err := s.UnsatCore()
	astTestMust(t, err)
	if !slices.Equal(core, []ast.ASTHandle{notFree}) {
		t.Fatal("lost the conflicting assumption", core)
	}

	astTestMust(t, s.Pop(2))
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(clause))
	check(lbool.True, notFree)
}
