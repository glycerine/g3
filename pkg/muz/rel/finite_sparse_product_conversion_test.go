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
	if canConvert, e := product.CanConvertToTableRelation(); e != nil || !canConvert {
		t.Fatalf("table conversion capability=%v err=%v", canConvert, e)
	}
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
	if canConvert, e := withInner.CanConvertToTableRelation(); e != nil || canConvert {
		t.Fatalf("inner-column conversion capability=%v err=%v", canConvert, e)
	}
	if _, err = withInner.ToTableRelation(); err == nil {
		t.Fatal("product with inner columns converted to table relation")
	}
	astTestMust(t, withInner.Close())
	if _, err = withInner.CanConvertToTableRelation(); err == nil {
		t.Fatal("closed product reported conversion capability")
	}
}

func TestFiniteSparseProductJoinTableRelationConversion(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-join-table-conversion")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 3)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one := value(0), value(1)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{sort})
	astTestMust(t, err)
	defer func() { astTestMust(t, table.Close()) }()
	if _, err = table.AddFact([]ast.ASTHandle{one}); err != nil {
		t.Fatal(err)
	}
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact([]ast.ASTHandle{one}))
	astTestMust(t, product.AddFact([]ast.ASTHandle{zero}))

	for _, tc := range []struct {
		name string
		hit  []ast.ASTHandle
		miss []ast.ASTHandle
	}{
		{name: "table right", hit: []ast.ASTHandle{one, one}, miss: []ast.ASTHandle{zero, one}},
		{name: "table left", hit: []ast.ASTHandle{one, one}, miss: []ast.ASTHandle{one, zero}},
	} {
		var result *FiniteSparseProduct
		if tc.name == "table right" {
			result, err = product.JoinTableRelation(table, []int{0}, []int{0})
		} else {
			result, err = product.JoinFromTableRelation(table, []int{0}, []int{0})
		}
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if found, e := result.ContainsFact(tc.hit); e != nil || !found {
			t.Fatalf("%s lost joined fact: %v (%v)", tc.name, found, e)
		}
		if found, e := result.ContainsFact(tc.miss); e != nil || found {
			t.Fatalf("%s admitted non-join fact: %v (%v)", tc.name, found, e)
		}
		astTestMust(t, result.Close())
	}
}

func TestFiniteSparseProductFromInnerSparseTableClonesInput(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-inner-conversion")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 3)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two := value(0), value(1), value(2)
	signature, err := NewSignature([]uint64{3, 3})
	astTestMust(t, err)
	source, err := NewSparseTable(signature, 0)
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	if _, err = source.Add(Fact{1, 2}); err != nil {
		t.Fatal(err)
	}
	product, err := NewFiniteSparseProductFromInnerSparseTable(f.relations, []ast.ASTHandle{sort, sort}, source)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	if found, e := product.ContainsFact([]ast.ASTHandle{one, two}); e != nil || !found {
		t.Fatalf("converted product lost source row: %v (%v)", found, e)
	}
	if _, err = source.Add(Fact{0, 0}); err != nil {
		t.Fatal(err)
	}
	if found, e := product.ContainsFact([]ast.ASTHandle{zero, zero}); e != nil || found {
		t.Fatalf("converted product aliases source rows: %v (%v)", found, e)
	}
	if _, err = NewFiniteSparseProductFromInnerSparseTable(f.relations, []ast.ASTHandle{sort}, source); err == nil {
		t.Fatal("accepted source signature with the wrong arity")
	}
}

func TestFiniteSparseProductJoinInnerSparseTableConversion(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-inner-join-conversion")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 3)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one := value(0), value(1)
	signature, err := NewSignature([]uint64{3})
	astTestMust(t, err)
	inner, err := NewSparseTable(signature, 0)
	astTestMust(t, err)
	defer func() { astTestMust(t, inner.Close()) }()
	if _, err = inner.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort}, []bool{true})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact([]ast.ASTHandle{zero}))
	astTestMust(t, product.AddFact([]ast.ASTHandle{one}))

	for _, tc := range []struct {
		name string
		join func() (*FiniteSparseProduct, error)
	}{
		{name: "inner right", join: func() (*FiniteSparseProduct, error) {
			return product.JoinInnerSparseTable(inner, []ast.ASTHandle{sort}, []int{0}, []int{0})
		}},
		{name: "inner left", join: func() (*FiniteSparseProduct, error) {
			return product.JoinFromInnerSparseTable(inner, []ast.ASTHandle{sort}, []int{0}, []int{0})
		}},
	} {
		joined, e := tc.join()
		if e != nil {
			t.Fatalf("%s: %v", tc.name, e)
		}
		if found, e := joined.ContainsFact([]ast.ASTHandle{one, one}); e != nil || !found {
			t.Fatalf("%s lost joined fact: %v (%v)", tc.name, found, e)
		}
		if found, e := joined.ContainsFact([]ast.ASTHandle{zero, one}); e != nil || found {
			t.Fatalf("%s admitted non-join fact: %v (%v)", tc.name, found, e)
		}
		astTestMust(t, joined.Close())
	}
}
