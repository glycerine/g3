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
	if _, err := engine.splitCore(nil); !errors.Is(err, errMaxResState) {
		t.Fatal("empty core", err)
	}
	engine.weights[p] = nil
	if _, err := engine.splitCore([]ast.ASTHandle{p}); !errors.Is(err, errMaxResState) {
		t.Fatal("missing weight", err)
	}
}
