package opt_test

// Source: src/opt/maxres.cpp end-to-end exact rational-weight engine.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestArithmeticSolverUsesMaxResForRationalWeights(t *testing.T) {
	manager, evaluator := sortMaxFixture(t)
	p := sortMaxBool(t, manager, "maxres-rational-p")
	if err := manager.IncRef(p); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(p)
	q := sortMaxBool(t, manager, "maxres-rational-q")
	if err := manager.IncRef(q); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(q)
	r := sortMaxBool(t, manager, "maxres-rational-r")
	if err := manager.IncRef(r); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(r)
	variables := []ast.ASTHandle{p, q, r}
	config := smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator}
	solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", config)
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
	weights := []*big.Rat{big.NewRat(5, 2), big.NewRat(3, 2), big.NewRat(1, 2)}
	if err = solver.AddSoftObjective(variables, weights); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if result.Status != lbool.True || !result.Optimal {
		t.Fatal("MaxRes status", result.Status, result.Optimal)
	}
	value := result.Values[0]
	if value.SoftCost.Cmp(big.NewRat(2, 1)) != 0 || value.SoftSatisfied.Cmp(big.NewRat(5, 2)) != 0 {
		t.Fatal("rational MaxRes optimum", &value.SoftCost, &value.SoftSatisfied)
	}
}

// AUDIT.md opt_01-F2: nothing in z3 4.7.1 calls
// maxsmt_solver_base::enable_sls, so opt.enable_sls=true leaves MaxRes
// unchanged: the same checks, the same assignment.
func TestMaxResEnableSLSIsInert(t *testing.T) {
	run := func(enabled bool) ([]bool, opt.MaxSMTStatistics) {
		manager, evaluator := sortMaxFixture(t)
		key := "enable_sls"
		if err := evaluator.Context.SetBoolName(evaluator.Params, &key, enabled); err != nil {
			t.Fatal(err)
		}
		p := sortMaxBool(t, manager, "maxres-sls-p")
		q := sortMaxBool(t, manager, "maxres-sls-q")
		r := sortMaxBool(t, manager, "maxres-sls-r")
		for _, expression := range []ast.ASTHandle{p, q, r} {
			if err := manager.IncRef(expression); err != nil {
				t.Fatal(err)
			}
			defer manager.DecRef(expression)
		}
		both, err := manager.MkAnd(p, q)
		if err != nil {
			t.Fatal(err)
		}
		mutex, err := manager.MkNot(both)
		if err != nil {
			t.Fatal(err)
		}
		either, err := manager.MkOr(q, r)
		if err != nil {
			t.Fatal(err)
		}
		for _, expression := range []ast.ASTHandle{both, mutex, either} {
			if err = manager.IncRef(expression); err != nil {
				t.Fatal(err)
			}
			defer manager.DecRef(expression)
		}
		solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
		if err != nil {
			t.Fatal(err)
		}
		defer solver.Close()
		for _, hard := range []ast.ASTHandle{mutex, either} {
			if err = solver.Assert(hard); err != nil {
				t.Fatal(err)
			}
		}
		if err = solver.AddSoftObjective([]ast.ASTHandle{p, q, r}, []*big.Rat{big.NewRat(5, 1), big.NewRat(3, 1), big.NewRat(1, 1)}); err != nil {
			t.Fatal(err)
		}
		result, err := solver.Check()
		if err != nil {
			t.Fatal(err)
		}
		defer result.Close()
		if result.Status != lbool.True || !result.Optimal || len(result.MaxSMT) != 1 {
			t.Fatal("MaxRes result", result.Status, result.Optimal, result.MaxSMT)
		}
		return append([]bool(nil), result.Values[0].SoftAssignment...), result.MaxSMT[0]
	}
	assignment, statistics := run(false)
	slsAssignment, slsStatistics := run(true)
	if len(assignment) != 3 || len(slsAssignment) != 3 {
		t.Fatal("soft assignments", assignment, slsAssignment)
	}
	for i := range assignment {
		if assignment[i] != slsAssignment[i] {
			t.Fatal("enable_sls changed the assignment", assignment, slsAssignment)
		}
	}
	if statistics != slsStatistics {
		t.Fatal("enable_sls changed the MaxRes run", statistics, slsStatistics)
	}
}
