package opt_test

// Source: src/opt/maxsmt.cpp find_mutexes/process_mutex retained by MaxRes.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestMaxResPreprocessesRationalMutexWeights(t *testing.T) {
	manager, evaluator := sortMaxFixture(t)
	p := sortMaxBool(t, manager, "maxres-rmutex-p")
	if err := manager.IncRef(p); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(p)
	q := sortMaxBool(t, manager, "maxres-rmutex-q")
	if err := manager.IncRef(q); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(q)
	both, err := manager.MkAnd(p, q)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.IncRef(both); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(both)
	mutex, err := manager.MkNot(both)
	if err != nil {
		t.Fatal(err)
	}
	config := smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator}
	solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.Assert(mutex); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoftObjective([]ast.ASTHandle{p, q}, []*big.Rat{big.NewRat(5, 2), big.NewRat(3, 2)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if result.Status != lbool.True || !result.Optimal || result.Values[0].SoftSatisfied.Cmp(big.NewRat(5, 2)) != 0 || result.Values[0].SoftCost.Cmp(big.NewRat(3, 2)) != 0 {
		t.Fatal("rational mutex optimum", result.Status, result.Values)
	}
	if len(result.MaxSMT) != 1 || result.MaxSMT[0].Engine != "maxres" || result.MaxSMT[0].Mutexes == 0 || result.MaxSMT[0].MutexTerms == 0 {
		t.Fatal("rational mutex statistics", result.MaxSMT)
	}
	assignment, err := solver.SoftAssignment(result, 0)
	if err != nil || len(assignment) != 2 || !assignment[0] || assignment[1] {
		t.Fatal("rational mutex assignment", assignment, err)
	}
}
