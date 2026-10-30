package opt_test

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestMaxResSearchPublishesEngineResult(t *testing.T) {
	manager, evaluator := priorityFixture(t)
	p := priorityBool(t, manager, "maxres-search-result-p")
	notP, err := manager.MkNot(p)
	if err != nil {
		t.Fatal(err)
	}
	solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.AddSoftObjective([]ast.ASTHandle{p, notP}, []*big.Rat{big.NewRat(2, 1), big.NewRat(1, 1)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if len(result.MaxSMT) != 1 || result.MaxSMT[0].Engine != "maxres" || result.Values[0].SoftCost.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatalf("result: %+v %+v", result.MaxSMT, result.Values[0])
	}
}
