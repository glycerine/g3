package opt

// Source: src/opt/maxres.cpp get_cores and minimize_core limits.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResCoreFiltersDuplicatesAndHonorsMinimizeLimit(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	p := maxResTestBool(t, manager, "maxres-core-p")
	q := maxResTestBool(t, manager, "maxres-core-q")
	unknown := maxResTestBool(t, manager, "maxres-core-unknown")
	engine := &maxResEngine{weights: map[ast.ASTHandle]*big.Rat{p: big.NewRat(2, 1), q: big.NewRat(1, 1)}, config: maxSMTConfig{maxCoreSize: 1}}
	core, err := engine.filterCore([]ast.ASTHandle{unknown, p, p, q})
	if err != nil || len(core) != 2 || core[0] != p || core[1] != q {
		t.Fatal("filtered core", core, err)
	}
	minimized, err := engine.minimizeCore(core)
	if err != nil || len(minimized) != 2 || engine.stats.CoreMinChecks != 0 {
		t.Fatal("core limit", minimized, engine.stats, err)
	}
}
