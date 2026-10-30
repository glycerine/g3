package opt_test

// Source: src/opt/sortmax.cpp fresh comparator network and model filtering.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestSortMaxPacketEncodesAndFiltersComparatorWires(t *testing.T) {
	manager, evaluator := priorityFixture(t)
	key := "maxsat_engine"
	name, err := manager.Symbols.Intern("sortmax")
	if err != nil {
		t.Fatal(err)
	}
	if err = evaluator.Context.SetSymbolName(evaluator.Params, &key, name); err != nil {
		t.Fatal(err)
	}
	variables := []ast.ASTHandle{priorityBool(t, manager, "packet-wire-a"), priorityBool(t, manager, "packet-wire-b"), priorityBool(t, manager, "packet-wire-c")}
	solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.AddSoftObjective(variables, []*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	statistics := result.MaxSMT[0]
	if statistics.Comparators != 3 || statistics.Auxiliaries != 6 || statistics.NetworkClauses != 18 || !result.ModelFiltered {
		t.Fatal("SortMax network", statistics, result.ModelFiltered)
	}
}
