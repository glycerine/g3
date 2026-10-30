package opt

// Source: src/opt/maxres.cpp correction-set/core pivot selection.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResPivotUsesOnlySmallerActiveCorrectionSet(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	a := maxResTestBool(t, manager, "maxres-pivot-a")
	b := maxResTestBool(t, manager, "maxres-pivot-b")
	c := maxResTestBool(t, manager, "maxres-pivot-c")
	inactive := maxResTestBool(t, manager, "maxres-pivot-inactive")
	engine := maxResEngine{
		config:     maxSMTConfig{pivotOnCorrectionSet: true},
		correction: []ast.ASTHandle{a, inactive, a},
		weights: map[ast.ASTHandle]*big.Rat{
			a: big.NewRat(1, 1), b: big.NewRat(1, 1), c: big.NewRat(1, 1),
		},
	}
	selected, yes := engine.pivotCorrection([]ast.ASTHandle{a, b, c})
	if !yes || len(selected) != 1 || selected[0] != a || engine.stats.Pivots != 1 {
		t.Fatal("correction pivot", selected, yes, engine.stats)
	}
	engine.correction = []ast.ASTHandle{a, b, c}
	if selected, yes = engine.pivotCorrection([]ast.ASTHandle{a, b, c}); yes || selected != nil {
		t.Fatal("non-smaller correction pivot", selected, yes)
	}
}
