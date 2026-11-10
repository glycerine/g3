package opt

// Source: src/opt/maxsmt.cpp rational process_mutex decomposition.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResMutexTransformPreservesTotalWithExactLowerOffset(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	a := maxResTestBool(t, manager, "maxres-rmutex-a")
	b := maxResTestBool(t, manager, "maxres-rmutex-b")
	c := maxResTestBool(t, manager, "maxres-rmutex-c")
	engine := maxResEngine{solver: &ArithmeticSolver{manager: manager}, owner: maxSMTASTOwner{manager: manager}}
	defer engine.close()
	newSoft := newSoftWeightMap(manager)
	for _, entry := range []struct {
		e ast.ASTHandle
		w *big.Rat
	}{{a, big.NewRat(7, 2)}, {b, big.NewRat(2, 1)}, {c, big.NewRat(2, 1)}} {
		if err := newSoft.insert(entry.e, entry.w); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.transformMutex([]ast.ASTHandle{c, a, b}, newSoft); err != nil {
		t.Fatal(err)
	}
	if engine.lower.Cmp(big.NewRat(4, 1)) != 0 || engine.stats.Mutexes != 1 || engine.stats.MutexTerms != 2 {
		t.Fatal("rational mutex lower", &engine.lower, engine.stats)
	}
	total := new(big.Rat)
	for _, term := range newSoft.entries() {
		total.Add(total, &term.weight)
	}
	if total.Cmp(big.NewRat(7, 2)) != 0 || new(big.Rat).Add(total, &engine.lower).Cmp(big.NewRat(15, 2)) != 0 {
		t.Fatal("rational mutex conservation", total, &engine.lower)
	}
}
