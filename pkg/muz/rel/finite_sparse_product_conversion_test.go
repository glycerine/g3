package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteSparseProductTableRelationRoundTrip(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-table-conversion")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two := value(0), value(1), value(2)
	source, err := NewTableRelation(f.relations, []ast.ASTHandle{sort, sort})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {two, zero}} {
		if _, err = source.AddFact(fact); err != nil {
			t.Fatal(err)
		}
	}
	product, err := NewFiniteSparseProductFromTableRelation(source)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {two, zero}} {
		if found, e := product.ContainsFact(fact); e != nil || !found {
			t.Fatalf("converted product lost %v: %v (%v)", fact, found, e)
		}
	}
	table, err := product.ToTableRelation()
	astTestMust(t, err)
	defer func() { astTestMust(t, table.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {two, zero}} {
		if found, e := table.ContainsFact(fact); e != nil || !found {
			t.Fatalf("converted table lost %v: %v (%v)", fact, found, e)
		}
	}
	if _, err = NewFiniteSparseProductFromTableRelation(nil); err == nil {
		t.Fatal("nil table relation accepted")
	}

	withInner, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, withInner.Close()) }()
	if _, err = withInner.ToTableRelation(); err == nil {
		t.Fatal("product with inner columns converted to table relation")
	}
}
