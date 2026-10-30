package opt

// Source: src/opt/maxres.cpp weighted hill-climb scheduling.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResHillClimbSchedulesWeightGroupsOrFullSet(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	a := maxResTestBool(t, manager, "maxres-hill-a")
	b := maxResTestBool(t, manager, "maxres-hill-b")
	c := maxResTestBool(t, manager, "maxres-hill-c")
	engine := maxResEngine{
		config: maxSMTConfig{hillClimb: true},
		active: []ast.ASTHandle{c, a, b},
		weights: map[ast.ASTHandle]*big.Rat{
			a: big.NewRat(5, 2), b: big.NewRat(5, 2), c: big.NewRat(1, 2),
		},
	}
	boundaries := engine.hillClimbBoundaries()
	if len(boundaries) != 2 || boundaries[0] != 2 || boundaries[1] != 3 {
		t.Fatal("hill-climb boundaries", boundaries)
	}
	if engine.active[0] != a || engine.active[1] != b || engine.active[2] != c {
		t.Fatal("weight order", engine.active)
	}
	engine.config.hillClimb = false
	boundaries = engine.hillClimbBoundaries()
	if len(boundaries) != 1 || boundaries[0] != 3 {
		t.Fatal("full-set boundary", boundaries)
	}
}
