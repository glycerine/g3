package opt

// Source: src/opt/maxres.cpp get_cores filtering and batch-size limit.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestMaxResCoreFiltersDuplicatesAndUsesSizeAsBatchLimit(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	p := maxResTestBool(t, manager, "maxres-core-p")
	q := maxResTestBool(t, manager, "maxres-core-q")
	unknown := maxResTestBool(t, manager, "maxres-core-unknown")
	engine := &maxResEngine{lower: testRat(0, 1), upper: testRat(0, 1), solver: &ArithmeticSolver{manager: newWeightedSoftFixture(t).manager}, weights: map[ast.ASTHandle]*numeral.Rational{p: testRatPtr(2, 1), q: testRatPtr(1, 1)}, config: maxSMTConfig{maxCoreSize: 1}}
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
	engine := &maxResEngine{lower: testRat(0, 1), upper: testRat(0, 1), solver: &ArithmeticSolver{manager: newWeightedSoftFixture(t).manager}, config: maxSMTConfig{satCoreEnabled: true}}
	got, status, err := engine.minimizeCore(core)
	if err != nil || status != lbool.True || len(got) != len(core) || got[0] != p || got[1] != q || engine.stats.CoreMinChecks != 0 {
		t.Fatal("SAT core mode changed the extracted core", got, engine.stats, err)
	}
}
