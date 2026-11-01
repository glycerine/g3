package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteSparseProductFactsAndClone(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	zero, one, two, three := value(0), value(1), value(2), value(3)
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	if empty, e := product.Empty(); e != nil || !empty {
		t.Fatalf("new product empty: got %v (%v)", empty, e)
	}
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {zero, two}, {one, one}} {
		if err = product.AddFact(fact); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		fact []ast.ASTHandle
		want bool
	}{{[]ast.ASTHandle{zero, one}, true}, {[]ast.ASTHandle{zero, three}, false}} {
		found, e := product.ContainsFact(tc.fact)
		if e != nil || found != tc.want {
			t.Fatalf("contains %v: got %v want %v (%v)", tc.fact, found, tc.want, e)
		}
	}
	if empty, e := product.Empty(); e != nil || empty {
		t.Fatalf("populated product empty: got %v (%v)", empty, e)
	}
	clone, err := product.Clone()
	astTestMust(t, err)
	defer func() { astTestMust(t, clone.Close()) }()
	if err = clone.AddFact([]ast.ASTHandle{zero, three}); err != nil {
		t.Fatal(err)
	}
	if found, err := product.ContainsFact([]ast.ASTHandle{zero, three}); err != nil || found {
		t.Fatal("clone mutation escaped into source", found, err)
	}
	if found, err := clone.ContainsFact([]ast.ASTHandle{zero, three}); err != nil || !found {
		t.Fatal("clone did not accept inner fact", found, err)
	}
}

func TestFiniteSparseProductRejectsInvalidConstructionAndFacts(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-invalid-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 2)
	astTestMust(t, err)
	if _, err = NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true}); err == nil {
		t.Fatal("mismatched column mask accepted")
	}
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	if err = product.AddFact(nil); err == nil {
		t.Fatal("wrong-arity fact accepted")
	}
}

func TestFiniteSparseProductUnionFrom(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-union-domain")
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
	target, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	source, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	astTestMust(t, target.AddFact([]ast.ASTHandle{zero, one}))
	astTestMust(t, source.AddFact([]ast.ASTHandle{zero, two}))
	astTestMust(t, source.AddFact([]ast.ASTHandle{one, one}))
	if err = target.UnionFrom(source); err != nil {
		t.Fatal(err)
	}
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {zero, two}, {one, one}} {
		if found, e := target.ContainsFact(fact); e != nil || !found {
			t.Fatalf("union missing %v: %v (%v)", fact, found, e)
		}
	}
	if found, e := target.ContainsFact([]ast.ASTHandle{one, two}); e != nil || found {
		t.Fatalf("union added absent row: %v (%v)", found, e)
	}
	if found, e := source.ContainsFact([]ast.ASTHandle{zero, one}); e != nil || found {
		t.Fatalf("union mutated source: %v (%v)", found, e)
	}
}
