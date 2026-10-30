package opt_test

// Source: src/opt/maxres.cpp fresh auxiliary filtering and model publication.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestMaxResFiltersFreshProxiesFromRepeatedPublishedModels(t *testing.T) {
	manager, evaluator := sortMaxFixture(t)
	p := sortMaxBool(t, manager, "maxres-filter-p")
	if err := manager.IncRef(p); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(p)
	q := sortMaxBool(t, manager, "maxres-filter-q")
	if err := manager.IncRef(q); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(q)
	complex, err := manager.MkOr(p, q)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.IncRef(complex); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(complex)
	solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.AddSoftObjective([]ast.ASTHandle{complex}, []*big.Rat{big.NewRat(1, 2)}); err != nil {
		t.Fatal(err)
	}
	for iteration := 0; iteration < 2; iteration++ {
		result, err := solver.Check()
		if err != nil {
			t.Fatal(iteration, err)
		}
		if !result.ModelFiltered || len(result.MaxSMT) != 1 || result.MaxSMT[0].Auxiliaries == 0 {
			t.Fatal("filtered MaxRes publication", iteration, result.ModelFiltered, result.MaxSMT)
		}
		constants, err := result.Model.NumConstants()
		if err != nil || constants > 2 {
			t.Fatal("fresh proxy leaked", iteration, constants, err)
		}
		labels, err := solver.ResultLabels(result)
		if err != nil || len(labels) != 0 {
			t.Fatal("published labels", labels, err)
		}
		if err = result.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
