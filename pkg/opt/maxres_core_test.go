package opt

// Source: src/opt/maxres.cpp get_cores filtering and batch-size limit.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResCoreFiltersDuplicatesAndUsesSizeAsBatchLimit(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	p := maxResTestBool(t, manager, "maxres-core-p")
	q := maxResTestBool(t, manager, "maxres-core-q")
	unknown := maxResTestBool(t, manager, "maxres-core-unknown")
	engine := &maxResEngine{weights: map[ast.ASTHandle]*big.Rat{p: big.NewRat(2, 1), q: big.NewRat(1, 1)}, config: maxSMTConfig{maxCoreSize: 1}}
	core, err := engine.filterCore([]ast.ASTHandle{unknown, p, p, q})
	if err != nil || len(core) != 2 || core[0] != p || core[1] != q {
		t.Fatal("filtered core", core, err)
	}
	if !engine.coreBatchLimitReached(1, len(core)) || engine.stats.CoreMinChecks != 0 {
		t.Fatal("core batch limit", engine.stats)
	}
}

func TestMaxResSATCoreModeSkipsCoreMinimization(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	p := maxResTestBool(t, manager, "maxres-sat-core-p")
	q := maxResTestBool(t, manager, "maxres-sat-core-q")
	core := []ast.ASTHandle{p, q}
	engine := &maxResEngine{config: maxSMTConfig{satCoreEnabled: true}}
	got, err := engine.minimizeCore(core)
	if err != nil || len(got) != len(core) || got[0] != p || got[1] != q || engine.stats.CoreMinChecks != 0 {
		t.Fatal("SAT core mode changed the extracted core", got, engine.stats, err)
	}
}
