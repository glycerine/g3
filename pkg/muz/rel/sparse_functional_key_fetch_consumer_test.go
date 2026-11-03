package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSparseFunctionalZeroKeyFetchDrivesFiniteProductConsumer(t *testing.T) {
	signature, err := NewSignature([]uint64{16})
	astTestMust(t, err)
	index, err := NewSparseTable(signature, 1)
	astTestMust(t, err)
	defer func() { astTestMust(t, index.Close()) }()
	if _, err = index.Add(Fact{7}); err != nil {
		t.Fatal(err)
	}
	suffix, found, err := index.FetchFunctionalKey(nil)
	if err != nil || !found || len(suffix) != 1 || suffix[0] != 7 {
		t.Fatal("zero-key functional fetch", suffix, found, err)
	}
	suffix[0] = 3
	again, found, err := index.FetchFunctionalKey(nil)
	if err != nil || !found || again[0] != 7 {
		t.Fatal("functional suffix was borrowed", again, found, err)
	}
	if _, _, err = index.FetchFunctionalKey(Fact{0}); err == nil {
		t.Fatal("zero-key index accepted a spurious key column")
	}

	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-zero-outer-key")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		handle, cause := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, cause)
		astTestPin(t, f.ast, handle)
		return handle
	}
	one, two := value(1), value(2)
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort}, []bool{false})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact([]ast.ASTHandle{one}))
	astTestMust(t, product.AddFact([]ast.ASTHandle{two}))
	if rows, cause := product.table.Len(); cause != nil || rows != 1 {
		t.Fatal("zero-key product duplicated its functional outer row", rows, cause)
	}
	for _, fact := range [][]ast.ASTHandle{{one}, {two}} {
		if present, cause := product.ContainsFact(fact); cause != nil || !present {
			t.Fatal("zero-key product lost inner fact", fact, present, cause)
		}
	}
}
