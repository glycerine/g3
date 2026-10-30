package opt

// Source: src/opt/maxres.cpp literal/new_assumption state.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResBaseAggregatesSortsAndRemovesAssumptions(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	a := maxResTestBool(t, manager, "maxres-base-a")
	b := maxResTestBool(t, manager, "maxres-base-b")
	engine := &maxResEngine{solver: &ArithmeticSolver{manager: manager}, weights: make(map[ast.ASTHandle]*big.Rat)}
	if err := engine.addAssumption(a, big.NewRat(2, 3)); err != nil {
		t.Fatal(err)
	}
	if err := engine.addAssumption(b, big.NewRat(5, 7)); err != nil {
		t.Fatal(err)
	}
	if err := engine.addAssumption(a, big.NewRat(1, 3)); err != nil {
		t.Fatal(err)
	}
	engine.sortAssumptions()
	if engine.active[0] != a || engine.weights[a].Cmp(big.NewRat(1, 1)) != 0 || engine.stats.Assumptions != 2 {
		t.Fatal("MaxRes base", engine.active, engine.weights, engine.stats)
	}
	removed, err := engine.removeAssumption(a)
	if err != nil || removed.Cmp(big.NewRat(1, 1)) != 0 || len(engine.active) != 1 {
		t.Fatal("MaxRes removal", removed, engine.active, err)
	}
}
