package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteIntervalProductZeroTableKeyUsesOneFunctionalRow(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	numeral := func(text string) ast.ASTHandle {
		value, cause := f.ast.ArithmeticIntegerNumeralText(text)
		astTestMust(t, cause)
		astTestPin(t, f.ast, value)
		return value
	}
	one, two, three := numeral("1"), numeral("2"), numeral("3")
	product, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{integer}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()

	astTestMust(t, product.AddFact([]ast.ASTHandle{one}))
	astTestMust(t, product.AddFact([]ast.ASTHandle{three}))
	if rows, cause := product.table.Len(); cause != nil || rows != 1 {
		t.Fatal("all-interval product duplicated its empty functional key", rows, cause)
	}
	for _, value := range []ast.ASTHandle{one, two, three} {
		if present, cause := product.ContainsFact([]ast.ASTHandle{value}); cause != nil || !present {
			t.Fatal("functional-key interval hull lost value", value, present, cause)
		}
	}
}
