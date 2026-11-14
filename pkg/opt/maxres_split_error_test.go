package opt

// Source: src/opt/maxres.cpp split_core state guards.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResSplitCoreRejectsEmptyAndMissingWeights(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	p := maxResTestBool(t, manager, "maxres-split-missing")
	engine := &maxResEngine{weights: make(map[ast.ASTHandle]*big.Rat)}
	// split_core: if (core.empty()) return rational(0); (AUDIT.md FUZZ_J-F2)
	if weight, err := engine.splitCore(nil); err != nil || weight.Sign() != 0 {
		t.Fatal("empty core", weight, err)
	}
	engine.weights[p] = nil
	if _, err := engine.splitCore([]ast.ASTHandle{p}); !errors.Is(err, errMaxResState) {
		t.Fatal("missing weight", err)
	}
}
