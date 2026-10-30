package opt

// Source: src/opt/maxres.cpp mus_solver end-to-end without global dispatch.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestMaxResSearchFindsRationalWeightedOptimumWithoutRegistration(t *testing.T) {
	manager, fixture := pbSLSFixture(t)
	p := maxResTestBool(t, manager, "maxres-search-p")
	q := maxResTestBool(t, manager, "maxres-search-q")
	r := maxResTestBool(t, manager, "maxres-search-r")
	variables := []ast.ASTHandle{p, q, r}
	config := smtcore.SMTSolverConfig{Params: fixture.Context, Globals: fixture.Globals, ModelEvaluator: fixture.Model}
	solver, err := NewArithmeticSolver(manager, fixture.Model.Params, "", config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	for i := 0; i < len(variables); i++ {
		for j := i + 1; j < len(variables); j++ {
			both, err := manager.MkAnd(variables[i], variables[j])
			if err != nil {
				t.Fatal(err)
			}
			mutex, err := manager.MkNot(both)
			if err != nil {
				t.Fatal(err)
			}
			if err = solver.Assert(mutex); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = solver.AddSoftObjective(variables, []*big.Rat{big.NewRat(5, 2), big.NewRat(3, 2), big.NewRat(1, 2)}); err != nil {
		t.Fatal(err)
	}
	status, err := solver.backend.CheckSat(nil)
	if err != nil || status != lbool.True {
		t.Fatal("initial MaxRes model", status, err)
	}
	result := &ArithmeticResult{Values: make([]ObjectiveValue, 1)}
	defer result.Close()
	if err = solver.replaceModel(result); err != nil {
		t.Fatal(err)
	}
	engine, err := newMaxResEngine(solver, &solver.objectives[0], 0, maxSMTConfig{hillClimb: true, pivotOnCorrectionSet: true})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.close()
	status, optimal, err := engine.optimize(result)
	if err != nil || status != lbool.False || !optimal {
		t.Fatal("MaxRes search", status, optimal, err)
	}
	value := result.Values[0]
	if value.SoftCost.Cmp(big.NewRat(2, 1)) != 0 || value.SoftSatisfied.Cmp(big.NewRat(5, 2)) != 0 {
		t.Fatal("MaxRes optimum", &value.SoftCost, &value.SoftSatisfied)
	}
}
