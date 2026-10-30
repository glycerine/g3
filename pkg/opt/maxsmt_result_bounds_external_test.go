package opt_test

// Source: src/opt/opt_context.cpp lex MaxSMT lower/upper publication.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestLexSoftResultsPublishTightBoundsAndAssignments(t *testing.T) {
	manager, evaluator := priorityFixture(t)
	p := priorityBool(t, manager, "bounds-p")
	q := priorityBool(t, manager, "bounds-q")
	both, err := manager.MkAnd(p, q)
	if err != nil {
		t.Fatal(err)
	}
	mutex, err := manager.MkNot(both)
	if err != nil {
		t.Fatal(err)
	}
	solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.Assert(mutex); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoftObjective([]ast.ASTHandle{p}, []*big.Rat{big.NewRat(2, 1)}); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoftObjective([]ast.ASTHandle{q}, []*big.Rat{big.NewRat(3, 1)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	for i := 0; i < 2; i++ {
		lower, upper, err := solver.SoftBounds(result, i)
		if err != nil || lower.Cmp(upper) != 0 || lower.Cmp(&result.Values[i].SoftCost) != 0 {
			t.Fatal("soft bounds", i, lower, upper, result.Values[i], err)
		}
	}
}
