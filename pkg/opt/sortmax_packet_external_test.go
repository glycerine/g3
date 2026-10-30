package opt_test

// Source: src/opt/sortmax.cpp retained weighted engine packet.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSortMaxPacketOptimizesWeightedMutex(t *testing.T) {
	manager, evaluator := priorityFixture(t)
	key := "maxsat_engine"
	name, err := manager.Symbols.Intern("sortmax")
	if err != nil {
		t.Fatal(err)
	}
	if err = evaluator.Context.SetSymbolName(evaluator.Params, &key, name); err != nil {
		t.Fatal(err)
	}
	p := priorityBool(t, manager, "packet-sort-p")
	q := priorityBool(t, manager, "packet-sort-q")
	r := priorityBool(t, manager, "packet-sort-r")
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
	if err = solver.AddSoftObjective([]ast.ASTHandle{p, q, r}, []*big.Rat{big.NewRat(5, 1), big.NewRat(3, 1), big.NewRat(2, 1)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if result.Status != lbool.True || !result.Optimal || result.Values[0].SoftCost.Cmp(big.NewRat(3, 1)) != 0 || len(result.MaxSMT) != 1 || result.MaxSMT[0].Engine != "sortmax" || result.MaxSMT[0].Mutexes == 0 {
		t.Fatal("SortMax packet result", result.Status, result.Optimal, result.Values[0], result.MaxSMT)
	}
}
