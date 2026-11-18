package opt

// Source: src/opt/maxsmt.cpp rational process_mutex decomposition.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResMutexTransformPreservesTotalWithExactLowerOffset(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	a := maxResTestBool(t, manager, "maxres-rmutex-a")
	b := maxResTestBool(t, manager, "maxres-rmutex-b")
	c := maxResTestBool(t, manager, "maxres-rmutex-c")
	engine := maxResEngine{lower: testRat(0, 1), upper: testRat(0, 1), solver: &ArithmeticSolver{manager: manager}, trail: maxSMTASTOwner{manager: manager}, defOwner: maxSMTASTOwner{manager: manager}, baseTrail: maxSMTASTOwner{manager: manager}}
	defer engine.close()
	newSoft := newSoftWeightMap(manager)
	defer newSoft.release()
	for _, entry := range []struct {
		e ast.ASTHandle
		w *numeral.Rational
	}{{a, testRatPtr(7, 2)}, {b, testRatPtr(2, 1)}, {c, testRatPtr(2, 1)}} {
		if err := newSoft.insert(entry.e, entry.w); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.transformMutex([]ast.ASTHandle{c, a, b}, newSoft); err != nil {
		t.Fatal(err)
	}
	if !ratIs(&engine.lower, 4, 1) || engine.stats.Mutexes != 1 || engine.stats.MutexTerms != 2 {
		t.Fatal("rational mutex lower", &engine.lower, engine.stats)
	}
	q := manager.NumeralService().Manager
	total := testRat(0, 1)
	entries, err := newSoft.entries()
	if err != nil {
		t.Fatal(err)
	}
	for _, term := range entries {
		if err = q.Add(&total, &term.weight, &total); err != nil {
			t.Fatal(err)
		}
	}
	sum := testRat(0, 1)
	if err = q.Add(&total, &engine.lower, &sum); err != nil {
		t.Fatal(err)
	}
	if !ratIs(&total, 7, 2) || !ratIs(&sum, 15, 2) {
		t.Fatal("rational mutex conservation", ratString(&total), ratString(&engine.lower))
	}
}
