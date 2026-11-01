package sls

// Source: src/tactic/sls/sls_evaluator.h:668-795.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
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

func TestSLSPrunedUpdateSharedBooleanGraph(t *testing.T) {
	engine, search, evaluator, tracker, manager, numbers := slsEngineFixture(t)
	p := slsBoolConst(t, manager, "prune-shared-p")
	q := slsBoolConst(t, manager, "prune-shared-q")
	r := slsBoolConst(t, manager, "prune-shared-r")
	left, err := manager.MkOr(p, q)
	if err != nil {
		t.Fatal(err)
	}
	right, err := manager.MkOr(p, r)
	if err != nil {
		t.Fatal(err)
	}
	root, err := manager.MkAnd(left, right)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, p, q, r, left, right, root)
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
	one := numeral.NewInt(1)
	benefit, err := evaluator.UpdatePrune(search, decl, &one)
	if err != nil || !benefit {
		t.Fatal("shared candidate", benefit, err)
	}
	for _, node := range []ast.ASTHandle{left, right, root} {
		if score, err := tracker.Score(node); err != nil || score != 1 {
			t.Fatal("shared score", node, score, err)
		}
		value, err := tracker.GetValue(node)
		if err != nil {
			t.Fatal(err)
		}
		if zero, err := numbers.IsZero(value); err != nil || !zero {
			t.Fatal("shared Boolean value changed", node, zero, err)
		}
	}
	// Source paws_init is 40, so a true root contributes weight 40.
	if total, err := tracker.TopSum(); err != nil || total != 40 {
		t.Fatal("shared root score was counted more than once", total, err)
	}
}

func TestSLSPrunedUpdateAllocationFrontiers(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 256 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			engine, search, evaluator, _, manager, numbers := slsEngineFixture(t)
			if err := manager.RegisterDeclarationPlugins(); err != nil {
				t.Fatal(err)
			}
			sort, err := manager.BitVectorSort(256)
			if err != nil {
				t.Fatal(err)
			}
			x := slsBVConst(t, manager, sort, "prune-failure-x")
			one := slsBVNumeral(t, manager, 1, 256)
			two := slsBVNumeral(t, manager, 2, 256)
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
			unit, candidate := numeral.NewInt(1), numeral.NewInt(0)
			if err = numbers.Mul2k(&unit, 200, &candidate); err != nil {
				t.Fatal(err)
			}
			defer func() {
				manager.Budget().FailAfter(-1)
				if err := numbers.Del(&candidate); err != nil {
					t.Error(err)
				}
			}()
			manager.Budget().FailAfter(stop)
			_, err = evaluator.UpdatePrune(search, decl, &candidate)
			manager.Budget().FailAfter(-1)
			if err == nil {
				succeeded = true
				return
			}
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				if err := construction.Cleanup(); err != nil {
					t.Fatal(err)
				}
			}
			var memout memory.Failure
			if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &memout) || memout.Kind != memory.FailureMemout) {
				t.Fatal("unexpected pruning failure", err)
			}
			failures++
		})
		if t.Failed() {
			t.Fatal("pruned update allocation frontier", stop)
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("did not reach failure and success", failures, succeeded)
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
