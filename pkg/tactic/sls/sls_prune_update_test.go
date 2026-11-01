package sls

// Source: src/tactic/sls/sls_evaluator.h:668-795.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSLSPrunedUpdateDefersBooleanValuesAndStopsUnhelpfulPropagation(t *testing.T) {
	engine, search, evaluator, tracker, manager, numbers := slsEngineFixture(t)
	p := slsBoolConst(t, manager, "prune-update-p")
	q := slsBoolConst(t, manager, "prune-update-q")
	root, err := manager.MkOr(p, q)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, p, q, root)
	if err = engine.AssertExpression(root); err != nil {
		t.Fatal(err)
	}
	if err = search.Initialize([]ast.ASTHandle{root}); err != nil {
		t.Fatal(err)
	}
	if err = search.Reset(); err != nil {
		t.Fatal(err)
	}
	if _, err = engine.Rescore(); err != nil {
		t.Fatal(err)
	}
	decl, err := manager.GetDecl(p)
	if err != nil {
		t.Fatal(err)
	}
	one, zero := numeral.NewInt(1), numeral.NewInt(0)
	benefit, err := evaluator.UpdatePrune(search, decl, &one)
	if err != nil || !benefit {
		t.Fatal("improving candidate", benefit, err)
	}
	value, err := tracker.GetValue(root)
	if err != nil {
		t.Fatal(err)
	}
	isZero, err := numbers.IsZero(value)
	if err != nil || !isZero {
		t.Fatal("pruned update changed the Boolean root value", isZero, err)
	}
	if score, err := tracker.Score(root); err != nil || score != 1 {
		t.Fatal("improving root score", score, err)
	}
	benefit, err = evaluator.UpdatePrune(search, decl, &zero)
	if err != nil || benefit {
		t.Fatal("non-improving candidate", benefit, err)
	}
	if score, err := tracker.Score(root); err != nil || score != 1 {
		t.Fatal("unhelpful candidate propagated to the root", score, err)
	}
	if score, err := engine.incrementalScore(decl, &zero); err != nil || score != 0 {
		t.Fatal("full restoration", score, err)
	}
}

func TestSLSPrunedUpdatePropagatesBitVectorValues(t *testing.T) {
	engine, search, evaluator, tracker, manager, numbers := slsEngineFixture(t)
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort, err := manager.BitVectorSort(4)
	if err != nil {
		t.Fatal(err)
	}
	x := slsBVConst(t, manager, sort, "prune-update-x")
	one := slsBVNumeral(t, manager, 1, 4)
	two := slsBVNumeral(t, manager, 2, 4)
	sum, err := manager.MkBitVectorAdd(x, one)
	if err != nil {
		t.Fatal(err)
	}
	root, err := manager.MkEq(sum, two)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, x, one, two, sum, root)
	if err = engine.AssertExpression(root); err != nil {
		t.Fatal(err)
	}
	if err = search.Initialize([]ast.ASTHandle{root}); err != nil {
		t.Fatal(err)
	}
	if err = search.Reset(); err != nil {
		t.Fatal(err)
	}
	if _, err = engine.Rescore(); err != nil {
		t.Fatal(err)
	}
	decl, err := manager.GetDecl(x)
	if err != nil {
		t.Fatal(err)
	}
	value := numeral.NewInt(1)
	benefit, err := evaluator.UpdatePrune(search, decl, &value)
	if err != nil || !benefit {
		t.Fatal("bit-vector candidate", benefit, err)
	}
	sumValue, err := tracker.GetValue(sum)
	if err != nil || slsIntText(t, numbers, sumValue) != "2" {
		t.Fatal("bit-vector intermediate", err)
	}
	rootValue, err := tracker.GetValue(root)
	if err != nil {
		t.Fatal(err)
	}
	if zero, err := numbers.IsZero(rootValue); err != nil || !zero {
		t.Fatal("Boolean root value changed", zero, err)
	}
	if score, err := tracker.Score(root); err != nil || score != 1 {
		t.Fatal("Boolean root score", score, err)
	}
}

func TestSLSPrunedUpdateNegativeOccurrenceUsesBaseline(t *testing.T) {
	engine, search, evaluator, tracker, manager, _ := slsEngineFixture(t)
	p := slsBoolConst(t, manager, "prune-negative-p")
	root, err := manager.MkNot(p)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, p, root)
	if err = engine.AssertExpression(root); err != nil {
		t.Fatal(err)
	}
	if err = search.Initialize([]ast.ASTHandle{root}); err != nil {
		t.Fatal(err)
	}
	if err = search.Reset(); err != nil {
		t.Fatal(err)
	}
	if _, err = engine.Rescore(); err != nil {
		t.Fatal(err)
	}
	decl, err := manager.GetDecl(p)
	if err != nil {
		t.Fatal(err)
	}
	one, zero := numeral.NewInt(1), numeral.NewInt(0)
	benefit, err := evaluator.UpdatePrune(search, decl, &one)
	if err != nil || benefit {
		t.Fatal("worsening negative occurrence", benefit, err)
	}
	if score, err := tracker.Score(root); err != nil || score != 1 {
		t.Fatal("worsening candidate propagated", score, err)
	}
	benefit, err = evaluator.UpdatePrune(search, decl, &zero)
	if err != nil || !benefit {
		t.Fatal("negative occurrence at baseline must be considered", benefit, err)
	}
}
