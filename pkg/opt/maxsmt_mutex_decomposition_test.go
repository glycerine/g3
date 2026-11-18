package opt

// Source: src/opt/maxsmt.cpp weighted mutex decomposition.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxSMTMutexDecompositionPreservesTotalAndLowerOffset(t *testing.T) {
	fixture := newWeightedSoftFixture(t)
	sort, err := fixture.manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	name, err := fixture.manager.Symbols.Intern("mutex-r")
	if err != nil {
		t.Fatal(err)
	}
	r, err := fixture.manager.MkConst(name, sort)
	if err != nil {
		t.Fatal(err)
	}
	terms := []maxSMTWeightedTerm{{expression: fixture.p, weight: testRat(7, 1)}, {expression: fixture.q, weight: testRat(4, 1)}, {expression: r, weight: testRat(4, 1)}}
	d, err := newMaxSMTMutexDecomposition(fixture.manager, terms)
	defer d.close()
	if err != nil {
		t.Fatal(err)
	}
	if err = d.transform([]ast.ASTHandle{fixture.q, fixture.p, r}); err != nil {
		t.Fatal(err)
	}
	result, err := d.terms()
	if err != nil {
		t.Fatal(err)
	}
	q := fixture.manager.NumeralService().Manager
	total := testRat(0, 1)
	if err = q.Copy(&total, &d.offset); err != nil {
		t.Fatal(err)
	}
	for i := range result {
		if err = q.Add(&total, &result[i].weight, &total); err != nil {
			t.Fatal(err)
		}
	}
	if !ratIs(&d.offset, 8, 1) || !ratIs(&total, 15, 1) || len(result) != 2 {
		t.Fatal("mutex decomposition", ratString(&d.offset), ratString(&total), result)
	}
}
