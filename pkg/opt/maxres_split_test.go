package opt

// Source: src/opt/maxres.cpp split_core weighted residual behavior.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResSplitCoreRetainsOnlyWeightResiduals(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	p := maxResTestBool(t, manager, "maxres-split-p")
	q := maxResTestBool(t, manager, "maxres-split-q")
	engine := maxResEngine{solver: &ArithmeticSolver{manager: manager}, trail: maxSMTASTOwner{manager: manager}, defOwner: maxSMTASTOwner{manager: manager}, baseTrail: maxSMTASTOwner{manager: manager}, weights: make(map[ast.ASTHandle]*big.Rat)}
	defer engine.close()
	if err := engine.addAssumption(p, big.NewRat(5, 2)); err != nil {
		t.Fatal(err)
	}
	if err := engine.addAssumption(q, big.NewRat(3, 2)); err != nil {
		t.Fatal(err)
	}
	minimum, err := engine.splitCore([]ast.ASTHandle{p, q})
	if err != nil {
		t.Fatal(err)
	}
	if minimum.Cmp(big.NewRat(3, 2)) != 0 || len(engine.active) != 1 || engine.active[0] != p || engine.weights[p].Cmp(big.NewRat(1, 1)) != 0 || engine.weights[q] != nil {
		t.Fatal("core residual", minimum, engine.active, engine.weights)
	}
}
