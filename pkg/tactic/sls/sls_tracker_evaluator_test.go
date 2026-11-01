package sls

// Source: src/tactic/sls/sls_tracker.h:43-298 and
// src/tactic/sls/sls_evaluator.h:25-116.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func slsTrackerFixture(t *testing.T, trace ...*diagnostic.TraceTags) (*Tracker, *ast.ASTManager, *numeral.IntegerManager, *powers, *uparams.ParamsContext, *uparams.GlobalParamsService, *uparams.ParamsRef, *arena.Budget) {
	t.Helper()
	context, globals, local, budget := slsParamsFixture(t)
	config := ast.ASTManagerConfig{Symbols: context.SymbolsField, Numerals: context.NumeralsField, Budget: budget}
	if len(trace) > 0 {
		config.TraceTags = trace[0]
	}
	manager, err := ast.NewASTManager(config)
	if err != nil {
		t.Fatal(err)
	}
	numbers, err := numeral.NewIntegerManager(numeral.IntegerConfig{
		Cells: arena.Config{PageSize: 2, Budget: budget},
		Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: budget}},
	})
	if err != nil {
		t.Fatal(err)
	}
	powerCache, err := newPowers(numbers)
	if err != nil {
		t.Fatal(err)
	}
	tracker, err := NewTracker(manager, numbers, powerCache, context, globals)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		budget.FailAfter(-1)
		if err := tracker.Close(); err != nil {
			t.Error(err)
		}
		if err := powerCache.closeChecked(); err != nil {
			t.Error(err)
		}
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
		if err := numbers.CloseChecked(); err != nil {
			t.Error(err)
		}
	})
	return tracker, manager, numbers, powerCache, context, globals, local, budget
}

func slsPin(t *testing.T, manager *ast.ASTManager, handles ...ast.ASTHandle) {
	t.Helper()
	for _, handle := range handles {
		if err := manager.IncRef(handle); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for i := len(handles); i > 0; i-- {
			if err := manager.DecRef(handles[i-1]); err != nil {
				t.Error(err)
			}
		}
	})
}

func slsBoolConst(t *testing.T, manager *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	sort, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	symbol, err := manager.Symbols.Intern(name)
	if err != nil {
		t.Fatal(err)
	}
	value, err := manager.MkConst(symbol, sort)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func slsIntText(t *testing.T, numbers *numeral.IntegerManager, value *numeral.Int) string {
	t.Helper()
	text, err := numbers.String(value)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func TestSLSTrackerFoundationParamsStateAndAccessors(t *testing.T) {
	tracker, manager, numbers, _, context, _, local, _ := slsTrackerFixture(t)
	p := slsBoolConst(t, manager, "sls-foundation-p")
	q := slsBoolConst(t, manager, "sls-foundation-q")
	parent, err := manager.MkAnd(p, q)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, p, q, parent)

	forget, trackUnsat := "walksat_ucb_forget", "track_unsat"
	if err = context.SetDoubleName(local, &forget, 0.5); err != nil {
		t.Fatal(err)
	}
	if err = context.SetBoolName(local, &trackUnsat, true); err != nil {
		t.Fatal(err)
	}
	if err = tracker.UpdateParams(local); err != nil {
		t.Fatal(err)
	}
	if tracker.ucbForget != 0.5 || tracker.trackUnsat {
		t.Fatal("source parameter publication or forced track_unsat disable", tracker.ucbForget, tracker.trackUnsat)
	}

	one, zero := numeral.NewInt(1), numeral.NewInt(0)
	for _, h := range []ast.ASTHandle{p, q} {
		if _, err = tracker.trackExpr(h); err != nil {
			t.Fatal(err)
		}
	}
	if err = tracker.SetValue(parent, &one); !errors.Is(err, ErrTracker) {
		t.Fatal("source accessor admitted an expression missing from the score table", err)
	}
	if err = tracker.SetValue(p, &one); err != nil {
		t.Fatal(err)
	}
	if err = tracker.SetValue(q, &zero); err != nil {
		t.Fatal(err)
	}
	if err = tracker.SetScore(p, 3.5); err != nil {
		t.Fatal(err)
	}
	if err = tracker.SetScorePrune(p, 2.25); err != nil {
		t.Fatal(err)
	}
	if err = tracker.setOccurrences(p, 2, 1); err != nil {
		t.Fatal(err)
	}
	if err = tracker.SetDistance(p, 7); err != nil {
		t.Fatal(err)
	}
	if score, err := tracker.Score(p); err != nil || score != 3.5 {
		t.Fatal("score", score, err)
	}
	if score, err := tracker.ScorePrune(p); err != nil || score != 2.25 {
		t.Fatal("prune", score, err)
	}
	if pos, neg, err := tracker.Occurrences(p); err != nil || pos != 2 || neg != 1 {
		t.Fatal("occurrences", pos, neg, err)
	}
	if distance, err := tracker.Distance(p); err != nil || distance != 7 {
		t.Fatal("distance", distance, err)
	}

	decl, err := manager.GetDecl(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = tracker.setEntryPoint(decl, p); err != nil {
		t.Fatal(err)
	}
	if got, err := tracker.EntryPoint(decl); err != nil || got != p {
		t.Fatal("entry point", got, err)
	}
	if err = tracker.SetDeclValue(decl, &zero); err != nil {
		t.Fatal(err)
	}
	if got, err := tracker.DeclValue(decl); err != nil || slsIntText(t, numbers, got) != "0" {
		t.Fatal("decl value", err)
	}
	if err = tracker.SetDeclScore(decl, 9.5); err != nil {
		t.Fatal(err)
	}
	if got, err := tracker.DeclScore(decl); err != nil || got != 9.5 {
		t.Fatal("decl score", got, err)
	}

	if err = tracker.addUplink(p, parent); err != nil {
		t.Fatal(err)
	}
	if yes, err := tracker.HasUplinks(p); err != nil || !yes {
		t.Fatal("uplink membership", yes, err)
	}
	if links, err := tracker.Uplinks(p); err != nil || len(links) != 1 || links[0] != parent {
		t.Fatal("uplinks", links, err)
	}
	if err = tracker.setTopExpr(p, 3); err != nil {
		t.Fatal(err)
	}
	if err = tracker.SetTopSum(10); err != nil {
		t.Fatal(err)
	}
	if err = tracker.AdaptTopSum(p, 4, 1); err != nil {
		t.Fatal(err)
	}
	if sum, err := tracker.TopSum(); err != nil || sum != 19 {
		t.Fatal("top sum", sum, err)
	}
	if sat, err := tracker.IsSAT(); err != nil || sat {
		t.Fatal("zero top expression", sat, err)
	}
	if err = tracker.SetValue(p, &one); err != nil {
		t.Fatal(err)
	}
	if sat, err := tracker.IsSAT(); err != nil || !sat {
		t.Fatal("one top expression", sat, err)
	}

	score := tracker.scores[p]
	score.touched, tracker.touched = 5, 5
	if err = tracker.ucbForgetExpressions([]ast.ASTHandle{p}); err != nil {
		t.Fatal(err)
	}
	if score.touched != 3 || tracker.touched != 3 {
		t.Fatal("ucb forgetting", score.touched, tracker.touched)
	}
}

func TestSLSEvaluatorBooleanFamilyAndBorrowedTrackerValues(t *testing.T) {
	tracker, manager, numbers, powerCache, _, _, _, _ := slsTrackerFixture(t)
	evaluator, err := NewEvaluator(manager, tracker, numbers, powerCache)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := evaluator.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	p := slsBoolConst(t, manager, "sls-eval-p")
	q := slsBoolConst(t, manager, "sls-eval-q")
	r := slsBoolConst(t, manager, "sls-eval-r")
	and, err := manager.MkAnd3(p, q, r)
	if err != nil {
		t.Fatal(err)
	}
	or, err := manager.MkOr3(p, q, r)
	if err != nil {
		t.Fatal(err)
	}
	not, err := manager.MkNot(q)
	if err != nil {
		t.Fatal(err)
	}
	ite, err := manager.MkIte(p, q, r)
	if err != nil {
		t.Fatal(err)
	}
	implies, err := manager.MkImplies(p, q)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, p, q, r, and, or, not, ite, implies)
	one, zero := numeral.NewInt(1), numeral.NewInt(0)
	for h, value := range map[ast.ASTHandle]*numeral.Int{p: &one, q: &zero, r: &one} {
		if _, err = tracker.trackExpr(h); err != nil {
			t.Fatal(err)
		}
		if err = tracker.SetValue(h, value); err != nil {
			t.Fatal(err)
		}
	}
	result := numeral.NewInt(0)
	defer func() {
		if err := numbers.Del(&result); err != nil {
			t.Fatal(err)
		}
	}()
	checks := []struct {
		node ast.ASTHandle
		want string
	}{{p, "1"}, {and, "0"}, {or, "1"}, {not, "1"}, {ite, "0"}}
	for _, check := range checks {
		if err = evaluator.Evaluate(check.node, &result); err != nil {
			t.Fatal(err)
		}
		if got := slsIntText(t, numbers, &result); got != check.want {
			t.Fatalf("evaluation %v = %s, want %s", check.node, got, check.want)
		}
	}

	sortName, err := manager.Symbols.Intern("SLS-Eval-U")
	if err != nil {
		t.Fatal(err)
	}
	uSort, err := manager.MkUninterpretedSort(sortName, nil)
	if err != nil {
		t.Fatal(err)
	}
	uName, _ := manager.Symbols.Intern("sls-eval-u")
	vName, _ := manager.Symbols.Intern("sls-eval-v")
	wName, _ := manager.Symbols.Intern("sls-eval-w")
	u, err := manager.MkConst(uName, uSort)
	if err != nil {
		t.Fatal(err)
	}
	v, err := manager.MkConst(vName, uSort)
	if err != nil {
		t.Fatal(err)
	}
	w, err := manager.MkConst(wName, uSort)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := manager.MkEq(u, v)
	if err != nil {
		t.Fatal(err)
	}
	distinct, err := manager.MkDistinct([]ast.ASTHandle{u, v, w})
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, uSort, u, v, w, equal, distinct)
	nine, ten := numeral.NewInt(9), numeral.NewInt(10)
	for h, value := range map[ast.ASTHandle]*numeral.Int{u: &nine, v: &nine, w: &ten} {
		if _, err = tracker.trackExpr(h); err != nil {
			t.Fatal(err)
		}
		if err = tracker.SetValue(h, value); err != nil {
			t.Fatal(err)
		}
	}
	if err = evaluator.Evaluate(equal, &result); err != nil || slsIntText(t, numbers, &result) != "1" {
		t.Fatal("equality", err)
	}
	if err = evaluator.Evaluate(distinct, &result); err != nil || slsIntText(t, numbers, &result) != "0" {
		t.Fatal("distinct", err)
	}
	if err = evaluator.Evaluate(implies, &result); !errors.Is(err, ErrEvaluatorUnsupported) {
		t.Fatal("unsupported implies", err)
	}
}

func TestSLSTrackerAndEvaluatorOwnerGuards(t *testing.T) {
	tracker, manager, numbers, powerCache, _, _, _, _ := slsTrackerFixture(t)
	copyTracker := *tracker
	if _, err := copyTracker.TopSum(); !errors.Is(err, ErrTracker) {
		t.Fatal("tracker copy", err)
	}
	evaluator, err := NewEvaluator(manager, tracker, numbers, powerCache)
	if err != nil {
		t.Fatal(err)
	}
	copyEvaluator := *evaluator
	if err = copyEvaluator.Evaluate(ast.ASTHandle{}, new(numeral.Int)); !errors.Is(err, ErrTracker) {
		t.Fatal("evaluator copy", err)
	}
	if err = evaluator.Close(); err != nil {
		t.Fatal(err)
	}
	if err = evaluator.Close(); err != nil {
		t.Fatal("idempotent evaluator close", err)
	}
	if err = tracker.Close(); err != nil {
		t.Fatal(err)
	}
	if err = tracker.Close(); err != nil {
		t.Fatal("idempotent tracker close", err)
	}
	if _, err = tracker.TopSum(); !errors.Is(err, ErrTracker) {
		t.Fatal("closed tracker", err)
	}
}

func TestSLSTrackerValueCopyFailurePrefix(t *testing.T) {
	tracker, manager, numbers, _, _, _, _, budget := slsTrackerFixture(t)
	p := slsBoolConst(t, manager, "sls-physical-prefix")
	slsPin(t, manager, p)
	if _, err := tracker.trackExpr(p); err != nil {
		t.Fatal(err)
	}
	huge := numeral.NewInt(0)
	one := numeral.NewInt(1)
	if err := numbers.Mul2k(&one, 10000, &huge); err != nil {
		t.Fatal(err)
	}
	defer func() {
		budget.FailAfter(-1)
		if err := numbers.Del(&huge); err != nil {
			t.Fatal(err)
		}
	}()

	budget.FailAfter(0)
	if err := tracker.SetValue(p, &huge); err == nil {
		t.Fatal("tracker copy did not reach physical allocation refusal")
	}
	if tracker.scores[p].value.Cell != numbers.FailedNullRef {
		t.Fatal("tracker did not preserve the source copy-failure prefix", tracker.scores[p].value)
	}
}

func TestSLSEvaluatorCopyFailurePrefix(t *testing.T) {
	tracker, manager, numbers, powerCache, _, _, _, budget := slsTrackerFixture(t)
	p := slsBoolConst(t, manager, "sls-evaluator-physical-prefix")
	slsPin(t, manager, p)
	if _, err := tracker.trackExpr(p); err != nil {
		t.Fatal(err)
	}
	huge := numeral.NewInt(0)
	one := numeral.NewInt(1)
	if err := numbers.Mul2k(&one, 10000, &huge); err != nil {
		t.Fatal(err)
	}
	defer func() {
		budget.FailAfter(-1)
		if err := numbers.Del(&huge); err != nil {
			t.Fatal(err)
		}
	}()
	if err := tracker.SetValue(p, &huge); err != nil {
		t.Fatal(err)
	}

	evaluator, err := NewEvaluator(manager, tracker, numbers, powerCache)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := evaluator.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	result := numeral.NewInt(0)
	defer func() {
		budget.FailAfter(-1)
		if err := numbers.Del(&result); err != nil {
			t.Fatal(err)
		}
	}()
	budget.FailAfter(0)
	if err = evaluator.Evaluate(p, &result); err == nil {
		t.Fatal("evaluator copy did not reach physical allocation refusal")
	}
	if result.Cell != numbers.FailedNullRef {
		t.Fatal("evaluator did not preserve the source copy-failure prefix", result)
	}
}
