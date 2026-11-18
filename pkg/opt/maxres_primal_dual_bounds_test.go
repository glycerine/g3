package opt

// Source: src/opt/maxres.cpp primal-dual weighted lower-bound clamp.

import (
	"testing"
)

func TestPrimalDualClampsOvershootingLowerBoundOnlyAfterCandidate(t *testing.T) {
	engine := &maxResEngine{solver: &ArithmeticSolver{manager: newWeightedSoftFixture(t).manager}, strategy: maxResPrimalDual, lower: testRat(9, 2), upper: testRat(3, 1)}
	engine.clampPrimalDualLower()
	if !ratIs(&engine.lower, 9, 2) {
		t.Fatal("clamped without candidate", &engine.lower)
	}
	engine.bestSet = true
	engine.clampPrimalDualLower()
	if !ratIs(&engine.lower, 3, 1) {
		t.Fatal("primal-dual clamp", &engine.lower)
	}
	engine.strategy = maxResPrimal
	engine.lower = testRat(5, 1)
	engine.clampPrimalDualLower()
	if !ratIs(&engine.lower, 5, 1) {
		t.Fatal("primal strategy clamp", &engine.lower)
	}
}
