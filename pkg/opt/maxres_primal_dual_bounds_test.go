package opt

// Source: src/opt/maxres.cpp primal-dual weighted lower-bound clamp.

import (
	"math/big"
	"testing"
)

func TestPrimalDualClampsOvershootingLowerBoundOnlyAfterCandidate(t *testing.T) {
	engine := &maxResEngine{strategy: maxResPrimalDual, lower: *big.NewRat(9, 2), upper: *big.NewRat(3, 1)}
	engine.clampPrimalDualLower()
	if engine.lower.Cmp(big.NewRat(9, 2)) != 0 {
		t.Fatal("clamped without candidate", &engine.lower)
	}
	engine.bestSet = true
	engine.clampPrimalDualLower()
	if engine.lower.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("primal-dual clamp", &engine.lower)
	}
	engine.strategy = maxResPrimal
	engine.lower.SetInt64(5)
	engine.clampPrimalDualLower()
	if engine.lower.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("primal strategy clamp", &engine.lower)
	}
}
