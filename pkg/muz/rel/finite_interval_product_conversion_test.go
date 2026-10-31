package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteIntervalProductTableConversion(t *testing.T) {
	f := newTableRelationFixture(t)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, table.Close()) }()
	_, err = table.AddFact([]ast.ASTHandle{f.truth, f.falsehood})
	astTestMust(t, err)
	_, err = table.AddFact([]ast.ASTHandle{f.falsehood, f.truth})
	astTestMust(t, err)
	product, err := FiniteIntervalProductFromTable(table)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	back, err := product.ToTableRelation()
	astTestMust(t, err)
	defer func() { astTestMust(t, back.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{f.truth, f.falsehood}, {f.falsehood, f.truth}} {
		found, err := back.ContainsFact(fact)
		if err != nil || !found {
			t.Fatal("round trip lost fact", fact, found, err)
		}
	}
	astTestMust(t, product.FilterEqual(0, f.truth))
	found, err := table.ContainsFact([]ast.ASTHandle{f.falsehood, f.truth})
	astTestMust(t, err)
	if !found {
		t.Fatal("converted product mutated original table")
	}
}

func TestFiniteIntervalProductIntervalConversion(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	inner := NewIntervalEmpty(2)
	astTestMust(t, inner.AddFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}))
	astTestMust(t, inner.AddFact([]*big.Rat{big.NewRat(3, 1), big.NewRat(3, 1)}))
	astTestMust(t, inner.FilterIdentical(0, 1))
	p, err := FiniteIntervalProductFromInterval(f.relations, []ast.ASTHandle{integer, integer}, inner)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	one, err := f.ast.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	astTestPin(t, f.ast, one)
	three, err := f.ast.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	astTestPin(t, f.ast, three)
	found, err := p.ContainsFact([]ast.ASTHandle{one, three})
	if err != nil || !found {
		t.Fatal("source interval clone behavior", found, err)
	}
	if _, err = p.ToTableRelation(); err == nil {
		t.Fatal("converted interval product to finite table")
	}
	empty, err := FiniteIntervalProductFromInterval(f.relations, []ast.ASTHandle{integer}, NewIntervalEmpty(1))
	astTestMust(t, err)
	defer func() { astTestMust(t, empty.Close()) }()
	if isEmpty, err := empty.Empty(); err != nil || !isEmpty {
		t.Fatal("empty interval conversion", isEmpty, err)
	}
}
