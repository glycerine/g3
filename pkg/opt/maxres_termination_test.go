package opt

import (
	"testing"
)

func TestMaxResTerminationUsesReachedLowerBound(t *testing.T) {
	engine := &maxResEngine{lower: testRat(0, 1), upper: testRat(0, 1), solver: &ArithmeticSolver{manager: newWeightedSoftFixture(t).manager}, bestSet: true}
	engine.lower = testRat(5, 1)
	engine.upper = testRat(5, 1)
	if !engine.optimalByBounds() {
		t.Fatal("equal bounds must terminate")
	}
	engine.upper = testRat(6, 1)
	if engine.optimalByBounds() {
		t.Fatal("open bound gap terminated")
	}
	engine.bestSet = false
	engine.lower = testRat(8, 1)
	if engine.optimalByBounds() {
		t.Fatal("missing candidate terminated")
	}
}
