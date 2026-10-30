package opt

// Source: src/opt/maxres.cpp next_index weighted hill-climb groups.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResWeightBoundariesKeepEqualWeightsTogether(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	a := maxResTestBool(t, manager, "maxres-boundary-a")
	b := maxResTestBool(t, manager, "maxres-boundary-b")
	c := maxResTestBool(t, manager, "maxres-boundary-c")
	d := maxResTestBool(t, manager, "maxres-boundary-d")
	engine := maxResEngine{
		active:  []ast.ASTHandle{a, b, c, d},
		weights: map[ast.ASTHandle]*big.Rat{a: big.NewRat(7, 2), b: big.NewRat(7, 2), c: big.NewRat(3, 2), d: big.NewRat(1, 2)},
	}
	engine.sortAssumptions()
	boundaries := engine.weightBoundaries()
	want := []int{2, 3, 4}
	if len(boundaries) != len(want) {
		t.Fatal(boundaries)
	}
	for i := range want {
		if boundaries[i] != want[i] {
			t.Fatal(boundaries, want)
		}
	}
}
