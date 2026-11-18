package opt_test

// Source: src/opt/wmax.cpp weighted-theory MaxSMT search.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestWMaxEngineOptimizesWeightedMutexAndPublishesBounds(t *testing.T) {
	manager, evaluator := priorityFixture(t)
	key := "maxsat_engine"
	name, err := manager.Symbols.Intern("wmax")
	if err != nil {
		t.Fatal(err)
	}
	if err = evaluator.Context.SetSymbolName(evaluator.Params, &key, name); err != nil {
		t.Fatal(err)
	}
	p := priorityBool(t, manager, "packet-wmax-p")
	q := priorityBool(t, manager, "packet-wmax-q")
	r := priorityBool(t, manager, "packet-wmax-r")
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
	if err = solver.AddSoftObjective([]ast.ASTHandle{p, q, r}, []numeral.Rational{testRat(5, 1), testRat(3, 1), testRat(2, 1)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if result.Status != lbool.True || !result.Optimal {
		t.Fatalf("WMax status/optimal: %v %t", result.Status, result.Optimal)
	}
	if len(result.Values) != 1 || !ratIs(&result.Values[0].SoftCost, 3, 1) || !ratIs(&result.Values[0].SoftSatisfied, 7, 1) {
		t.Fatalf("WMax objective values: %+v", result.Values)
	}
	if !result.Values[0].SoftBoundsValid || !ratIs(&result.Values[0].SoftLower, 3, 1) || !ratIs(&result.Values[0].SoftUpper, 3, 1) {
		t.Fatalf("WMax objective bounds: %+v", result.Values[0])
	}
	if len(result.MaxSMT) != 1 || result.MaxSMT[0].Engine != "wmax" || result.MaxSMT[0].Checks == 0 || result.MaxSMT[0].Mutexes == 0 {
		t.Fatalf("WMax statistics: %+v", result.MaxSMT)
	}
}
