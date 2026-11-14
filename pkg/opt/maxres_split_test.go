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
	if minimum.Cmp(big.NewRat(3, 2)) != 0 || len(engine.active) != 1 || engine.active[0] != p || engine.weights[p].Cmp(big.NewRat(1, 1)) != 0 || engine.weights[q] == nil || engine.weights[q].Cmp(big.NewRat(3, 2)) != 0 {
		// remove_core leaves m_asm2weight alone (AUDIT.md FUZZ_J-F2).
		t.Fatal("core residual", minimum, engine.active, engine.weights)
	}
}

// AUDIT.md FUZZ_J-F2: a member removed by process_sat keeps its weight, so
// a later core of the same batch that contains it still splits.
func TestMaxResSplitCoreAfterCorrectionKeepsWeights(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	p := maxResTestBool(t, manager, "maxres-split-kept-p")
	q := maxResTestBool(t, manager, "maxres-split-kept-q")
	engine := maxResEngine{solver: &ArithmeticSolver{manager: manager}, trail: maxSMTASTOwner{manager: manager}, defOwner: maxSMTASTOwner{manager: manager}, baseTrail: maxSMTASTOwner{manager: manager}, weights: make(map[ast.ASTHandle]*big.Rat)}
	defer engine.close()
	if err := engine.addAssumption(p, big.NewRat(4, 1)); err != nil {
		t.Fatal(err)
	}
	if err := engine.addAssumption(q, big.NewRat(2, 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.splitCore([]ast.ASTHandle{p}); err != nil {
		t.Fatal(err)
	}
	if len(engine.active) != 1 || engine.active[0] != q {
		t.Fatal("p not removed", engine.active)
	}
	minimum, err := engine.splitCore([]ast.ASTHandle{q, p})
	if err != nil {
		t.Fatal(err)
	}
	if minimum.Cmp(big.NewRat(2, 1)) != 0 || len(engine.active) != 1 || engine.active[0] != p || engine.weights[p].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("second core", minimum, engine.active, engine.weights)
	}
}
