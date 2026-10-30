package opt_test

// Source: src/opt/maxsmt.cpp default maxsat_engine dispatch.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestMaxSMTDefaultEngineIsMaxResForIntegralWeights(t *testing.T) {
	manager, evaluator := sortMaxFixture(t)
	p := sortMaxBool(t, manager, "maxsmt-default-p")
	if err := manager.IncRef(p); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(p)
	solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.AddSoftObjective([]ast.ASTHandle{p}, []*big.Rat{big.NewRat(1, 1)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if len(result.MaxSMT) != 1 || result.MaxSMT[0].Engine != "maxres" {
		t.Fatal("default MaxSMT engine", result.MaxSMT)
	}
}
