package opt

// Source: src/opt/maxsmt.cpp weighted mutex decomposition.

import (
	"math/big"
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
	terms := []maxSMTWeightedTerm{{expression: fixture.p, weight: *big.NewInt(7)}, {expression: fixture.q, weight: *big.NewInt(4)}, {expression: r, weight: *big.NewInt(4)}}
	d := newMaxSMTMutexDecomposition(fixture.manager, terms)
	defer d.close()
	if err = d.transform([]ast.ASTHandle{fixture.q, fixture.p, r}); err != nil {
		t.Fatal(err)
	}
	result := d.terms()
	total := new(big.Int).Set(d.lowerOffset())
	for i := range result {
		total.Add(total, &result[i].weight)
	}
	if d.lowerOffset().Int64() != 8 || total.Int64() != 15 || len(result) != 2 {
		t.Fatal("mutex decomposition", d.lowerOffset(), total, result)
	}
}
