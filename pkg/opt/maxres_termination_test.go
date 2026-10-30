package opt

import (
	"math/big"
	"testing"
)

func TestMaxResTerminationUsesReachedLowerBound(t *testing.T) {
	engine := &maxResEngine{bestSet: true}
	engine.lower.Set(big.NewRat(5, 1))
	engine.upper.Set(big.NewRat(5, 1))
	if !engine.optimalByBounds() {
		t.Fatal("equal bounds must terminate")
	}
	engine.upper.Set(big.NewRat(6, 1))
	if engine.optimalByBounds() {
		t.Fatal("open bound gap terminated")
	}
	engine.bestSet = false
	engine.lower.Set(big.NewRat(8, 1))
	if engine.optimalByBounds() {
		t.Fatal("missing candidate terminated")
	}
}
