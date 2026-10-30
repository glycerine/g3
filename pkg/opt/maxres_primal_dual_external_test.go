package opt_test

// Source: src/opt/maxres.cpp primal-dual correction-set pivot integration.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestMaxResPrimalDualCorrectionPivotFindsRationalOptimum(t *testing.T) {
	manager, evaluator := sortMaxFixture(t)
	p := sortMaxBool(t, manager, "maxres-pd-p")
	if err := manager.IncRef(p); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(p)
	q := sortMaxBool(t, manager, "maxres-pd-q")
	if err := manager.IncRef(q); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(q)
	r := sortMaxBool(t, manager, "maxres-pd-r")
	if err := manager.IncRef(r); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(r)
	all, err := manager.MkAndArgs([]ast.ASTHandle{p, q, r})
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.IncRef(all); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(all)
	notAll, err := manager.MkNot(all)
	if err != nil {
		t.Fatal(err)
	}
	config := smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator}
	solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.Assert(notAll); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoftObjective([]ast.ASTHandle{p, q, r}, []*big.Rat{big.NewRat(5, 2), big.NewRat(3, 2), big.NewRat(1, 2)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if result.Status != lbool.True || !result.Optimal {
		t.Fatal("primal-dual status", result.Status, result.Optimal)
	}
	value := result.Values[0]
	if value.SoftSatisfied.Cmp(big.NewRat(4, 1)) != 0 || value.SoftCost.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("primal-dual optimum", &value.SoftSatisfied, &value.SoftCost)
	}
}
