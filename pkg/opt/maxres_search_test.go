package opt

// Source: src/opt/maxres.cpp mus_solver end-to-end without global dispatch.

import (
	"fmt"
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
	if !value.SoftBoundsValid || value.SoftLower.Cmp(big.NewRat(2, 1)) != 0 || value.SoftUpper.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("MaxRes optimal bounds", value.SoftBoundsValid, &value.SoftLower, &value.SoftUpper)
	}
}

func TestMaxResInterruptedSearchPublishesCandidateBounds(t *testing.T) {
	manager, fixture := pbSLSFixture(t)
	variables := []ast.ASTHandle{
		maxResTestBool(t, manager, "maxres-interrupted-p"),
		maxResTestBool(t, manager, "maxres-interrupted-q"),
		maxResTestBool(t, manager, "maxres-interrupted-r"),
	}
	all, err := manager.MkAndArray(variables)
	if err != nil {
		t.Fatal(err)
	}
	notAll, err := manager.MkNot(all)
	if err != nil {
		t.Fatal(err)
	}
	config := smtcore.SMTSolverConfig{Params: fixture.Context, Globals: fixture.Globals, ModelEvaluator: fixture.Model}
	solver, err := NewArithmeticSolver(manager, fixture.Model.Params, "", config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.Assert(notAll); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoftObjective(variables, []*big.Rat{big.NewRat(5, 2), big.NewRat(3, 2), big.NewRat(1, 2)}); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.backend.CheckSat(nil); err != nil || status != lbool.True {
		t.Fatal("initial model", status, err)
	}
	result := &ArithmeticResult{Values: make([]ObjectiveValue, 1)}
	defer result.Close()
	if err = solver.replaceModel(result); err != nil {
		t.Fatal(err)
	}
	engine, err := newMaxResEngine(solver, &solver.objectives[0], 0, maxSMTConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.close()
	manager.ResourceLimit().Cancel()
	status, optimal, err := engine.optimize(result)
	manager.ResourceLimit().ResetCancel()
	if err != nil || status != lbool.Undef || optimal {
		t.Fatal("interrupted search", status, optimal, err)
	}
	value := &result.Values[0]
	if !value.SoftBoundsValid || value.SoftLower.Sign() != 0 || value.SoftUpper.Cmp(&value.SoftCost) != 0 || value.SoftUpper.Sign() <= 0 {
		t.Fatal("interrupted bounds", value.SoftBoundsValid, &value.SoftLower, &value.SoftUpper, &value.SoftCost)
	}
}

func TestMaxResCoreMinimizationMatchesSourceForMoreThanThirtyTwoLiterals(t *testing.T) {
	manager, fixture := pbSLSFixture(t)
	const count = 33
	terms := make([]ast.ASTHandle, count)
	weights := make([]*big.Rat, count)
	for index := range terms {
		terms[index] = maxResTestBool(t, manager, fmt.Sprintf("maxres-large-mus-%d", index))
		weights[index] = big.NewRat(1, 1)
	}
	all, err := manager.MkAndArray(terms)
	if err != nil {
		t.Fatal(err)
	}
	notAll, err := manager.MkNot(all)
	if err != nil {
		t.Fatal(err)
	}
	config := smtcore.SMTSolverConfig{Params: fixture.Context, Globals: fixture.Globals, ModelEvaluator: fixture.Model}
	solver, err := NewArithmeticSolver(manager, fixture.Model.Params, "", config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.Assert(notAll); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoftObjective(terms, weights); err != nil {
		t.Fatal(err)
	}
	engine, err := newMaxResEngine(solver, &solver.objectives[0], 0, maxSMTConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.close()
	core, status, err := engine.minimizeCore(terms)
	if err != nil || status != lbool.True {
		t.Fatal("source MUS status", status, err)
	}
	if len(core) != count || engine.stats.CoreMinChecks != count {
		t.Fatal("source MUS must check every core literal", len(core), engine.stats.CoreMinChecks)
	}
}
