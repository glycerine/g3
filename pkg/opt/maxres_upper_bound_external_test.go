package opt_test

// Source: src/opt/maxres.cpp add_upper_bound_block and termination proof.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestMaxResUpperBoundBlockProvesImprovedCandidateOptimal(t *testing.T) {
	manager, evaluator := sortMaxFixture(t)
	upperKey := "maxres.add_upper_bound_block"
	if err := evaluator.Context.SetBoolName(evaluator.Params, &upperKey, true); err != nil {
		t.Fatal(err)
	}
	p := sortMaxBool(t, manager, "maxres-upper-p")
	if err := manager.IncRef(p); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(p)
	q := sortMaxBool(t, manager, "maxres-upper-q")
	if err := manager.IncRef(q); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(q)
	r := sortMaxBool(t, manager, "maxres-upper-r")
	if err := manager.IncRef(r); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(r)
	config := smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator}
	solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	one, err := manager.ArithmeticIntegerNumeralText("1")
	if err != nil {
		t.Fatal(err)
	}
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.IncRef(one); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(one)
	if err = manager.IncRef(zero); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(zero)
	for _, other := range []ast.ASTHandle{q, r} {
		left, err := manager.MkIte(p, one, zero)
		if err != nil {
			t.Fatal(err)
		}
		if err = manager.IncRef(left); err != nil {
			t.Fatal(err)
		}
		right, err := manager.MkIte(other, one, zero)
		if err != nil {
			t.Fatal(err)
		}
		if err = manager.IncRef(right); err != nil {
			t.Fatal(err)
		}
		sum, err := manager.MkArithmeticAdd(left, right)
		if err != nil {
			t.Fatal(err)
		}
		if err = manager.IncRef(sum); err != nil {
			t.Fatal(err)
		}
		mutex, err := manager.MkArithmeticLE(sum, one)
		if err != nil {
			t.Fatal(err)
		}
		if err = solver.Assert(mutex); err != nil {
			t.Fatal(err)
		}
		if err = manager.DecRef(sum); err != nil {
			t.Fatal(err)
		}
		if err = manager.DecRef(right); err != nil {
			t.Fatal(err)
		}
		if err = manager.DecRef(left); err != nil {
			t.Fatal(err)
		}
	}
	if err = solver.AddSoftObjective([]ast.ASTHandle{p, q, r}, []*big.Rat{big.NewRat(5, 2), big.NewRat(3, 2), big.NewRat(3, 2)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if result.Status != lbool.True || !result.Optimal {
		t.Fatal("upper-bound status", result.Status, result.Optimal)
	}
	value := result.Values[0]
	if value.SoftSatisfied.Cmp(big.NewRat(3, 1)) != 0 || value.SoftCost.Cmp(big.NewRat(5, 2)) != 0 {
		t.Fatal("upper-bound optimum", &value.SoftSatisfied, &value.SoftCost)
	}
	if len(result.MaxSMT) != 1 || result.MaxSMT[0].Engine != "maxres" || result.MaxSMT[0].UpperBoundBlocks == 0 || result.MaxSMT[0].UpperBoundProofs == 0 {
		t.Fatal("upper-bound statistics", result.MaxSMT)
	}
}
