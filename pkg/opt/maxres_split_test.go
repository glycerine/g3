package opt

// Source: src/opt/maxres.cpp split_core weighted residual behavior.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResSplitCoreRetainsOnlyWeightResiduals(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	p := maxResTestBool(t, manager, "maxres-split-p")
	q := maxResTestBool(t, manager, "maxres-split-q")
	engine := maxResEngine{lower: testRat(0, 1), upper: testRat(0, 1), solver: &ArithmeticSolver{manager: manager}, trail: maxSMTASTOwner{manager: manager}, defOwner: maxSMTASTOwner{manager: manager}, baseTrail: maxSMTASTOwner{manager: manager}, weights: make(map[ast.ASTHandle]*numeral.Rational)}
	defer engine.close()
	if err := engine.addAssumption(p, testRatPtr(5, 2)); err != nil {
		t.Fatal(err)
	}
	if err := engine.addAssumption(q, testRatPtr(3, 2)); err != nil {
		t.Fatal(err)
	}
	minimum, err := engine.splitCore([]ast.ASTHandle{p, q})
	if err != nil {
		t.Fatal(err)
	}
	if !ratIs(&minimum, 3, 2) || len(engine.active) != 1 || engine.active[0] != p || !ratIs(engine.weights[p], 1, 1) || engine.weights[q] == nil || !ratIs(engine.weights[q], 3, 2) {
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
	engine := maxResEngine{lower: testRat(0, 1), upper: testRat(0, 1), solver: &ArithmeticSolver{manager: manager}, trail: maxSMTASTOwner{manager: manager}, defOwner: maxSMTASTOwner{manager: manager}, baseTrail: maxSMTASTOwner{manager: manager}, weights: make(map[ast.ASTHandle]*numeral.Rational)}
	defer engine.close()
	if err := engine.addAssumption(p, testRatPtr(4, 1)); err != nil {
		t.Fatal(err)
	}
	if err := engine.addAssumption(q, testRatPtr(2, 1)); err != nil {
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
	if !ratIs(&minimum, 2, 1) || len(engine.active) != 1 || engine.active[0] != p || !ratIs(engine.weights[p], 2, 1) {
		t.Fatal("second core", minimum, engine.active, engine.weights)
	}
}
