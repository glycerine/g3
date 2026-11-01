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

func TestFiniteSparseProductUnionFromWithDelta(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-delta-domain")
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
	newProduct := func() *FiniteSparseProduct {
		product, e := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
		astTestMust(t, e)
		return product
	}
	target, source, delta := newProduct(), newProduct(), newProduct()
	defer func() { astTestMust(t, target.Close()) }()
	defer func() { astTestMust(t, source.Close()) }()
	defer func() { astTestMust(t, delta.Close()) }()
	astTestMust(t, target.AddFact([]ast.ASTHandle{zero, one}))
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {zero, two}, {one, three}} {
		astTestMust(t, source.AddFact(fact))
	}
	astTestMust(t, delta.AddFact([]ast.ASTHandle{three, zero}))
	if err = target.UnionFromWithDelta(source, delta); err != nil {
		t.Fatal(err)
	}
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {zero, two}, {one, three}} {
		if found, e := target.ContainsFact(fact); e != nil || !found {
			t.Fatalf("union target missing %v: %v (%v)", fact, found, e)
		}
	}
	for _, fact := range [][]ast.ASTHandle{{zero, two}, {one, three}, {three, zero}} {
		if found, e := delta.ContainsFact(fact); e != nil || !found {
			t.Fatalf("union delta missing %v: %v (%v)", fact, found, e)
		}
	}
	flat, err := delta.flatten()
	astTestMust(t, err)
	if count, e := flat.Len(); e != nil || count != 3 {
		t.Fatalf("delta contains %d facts, want 3 (%v)", count, e)
	}
	astTestMust(t, flat.Close())
	if err = target.UnionFromWithDelta(source, delta); err != nil {
		t.Fatal(err)
	}
	flat, err = delta.flatten()
	astTestMust(t, err)
	if count, e := flat.Len(); e != nil || count != 3 {
		t.Fatalf("repeated union changed delta cardinality to %d (%v)", count, e)
	}
	astTestMust(t, flat.Close())
}

func TestFiniteSparseProductProjectMergesRows(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-project-domain")
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
	source, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort, sort}, []bool{true, false, true})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{zero, one, two}, {three, one, two}, {zero, two, three}} {
		astTestMust(t, source.AddFact(fact))
	}
	projected, err := source.Project([]int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{one, two}, {two, three}} {
		if found, e := projected.ContainsFact(fact); e != nil || !found {
			t.Fatalf("projected product missing %v: %v (%v)", fact, found, e)
		}
	}
	if found, e := projected.ContainsFact([]ast.ASTHandle{three, one}); e != nil || found {
		t.Fatalf("projection added absent row: %v (%v)", found, e)
	}
	for _, removed := range [][]int{{1, 0}, {3}} {
		if _, e := source.Project(removed); e == nil {
			t.Fatalf("invalid projection columns accepted: %v", removed)
		}
	}
}

func TestFiniteSparseProductJoinAcrossPartitions(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-join-domain")
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
	left, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, left.Close()) }()
	right, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{false, true})
	astTestMust(t, err)
	defer func() { astTestMust(t, right.Close()) }()
	astTestMust(t, left.AddFact([]ast.ASTHandle{zero, one}))
	astTestMust(t, left.AddFact([]ast.ASTHandle{one, two}))
	astTestMust(t, right.AddFact([]ast.ASTHandle{one, three}))
	astTestMust(t, right.AddFact([]ast.ASTHandle{two, zero}))
	joined, err := left.Join(right, []int{1}, []int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, joined.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{zero, one, one, three}, {one, two, two, zero}} {
		if found, e := joined.ContainsFact(fact); e != nil || !found {
			t.Fatalf("join missing %v: %v (%v)", fact, found, e)
		}
	}
	if found, e := joined.ContainsFact([]ast.ASTHandle{zero, one, two, zero}); e != nil || found {
		t.Fatalf("join produced a mismatched pair: %v (%v)", found, e)
	}
}

func TestFiniteSparseProductEqualityFilters(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-filter-domain")
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
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort, sort}, []bool{true, false, true})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{zero, zero, one}, {one, two, three}, {two, two, three}} {
		astTestMust(t, product.AddFact(fact))
	}
	if err = product.FilterIdentical([]int{0, 1}); err != nil {
		t.Fatal(err)
	}
	if err = product.FilterEqual(2, three); err != nil {
		t.Fatal(err)
	}
	for _, fact := range [][]ast.ASTHandle{{two, two, three}} {
		if found, e := product.ContainsFact(fact); e != nil || !found {
			t.Fatalf("equality filters removed matching row %v: %v (%v)", fact, found, e)
		}
	}
	for _, fact := range [][]ast.ASTHandle{{zero, zero, one}, {one, two, three}} {
		if found, e := product.ContainsFact(fact); e != nil || found {
			t.Fatalf("equality filters kept rejected row %v: %v (%v)", fact, found, e)
		}
	}
	otherName, err := f.ast.Symbols.Intern("finite-sparse-product-filter-other-domain")
	astTestMust(t, err)
	otherSort, err := f.ast.MkDatalogFiniteSort(otherName, 4)
	astTestMust(t, err)
	wrongSortValue, err := f.ast.MkDatalogNumeral(3, otherSort)
	astTestMust(t, err)
	astTestPin(t, f.ast, wrongSortValue)
	if err = product.FilterEqual(0, wrongSortValue); err == nil {
		t.Fatal("sort-mismatched value accepted")
	}
	crossSort, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, otherSort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, crossSort.Close()) }()
	astTestMust(t, crossSort.AddFact([]ast.ASTHandle{one, wrongSortValue}))
	if err = crossSort.FilterIdentical([]int{0, 1}); err != nil {
		t.Fatal(err)
	}
	if empty, e := crossSort.Empty(); e != nil || !empty {
		t.Fatalf("different-sort columns treated as identical: empty=%v err=%v", empty, e)
	}
}

func TestFiniteSparseProductNegationFilterAcrossPartitions(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-negation-domain")
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
	target, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	negative, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{false, true})
	astTestMust(t, err)
	defer func() { astTestMust(t, negative.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {one, two}, {one, three}} {
		astTestMust(t, target.AddFact(fact))
	}
	astTestMust(t, negative.AddFact([]ast.ASTHandle{one, three}))
	removed, err := target.FilterNegation(negative, []int{0, 1}, []int{0, 1})
	if err != nil || removed != 1 {
		t.Fatalf("negation removed %d rows, want 1 (%v)", removed, err)
	}
	for _, fact := range [][]ast.ASTHandle{{zero, one}, {one, two}} {
		if found, e := target.ContainsFact(fact); e != nil || !found {
			t.Fatalf("negation removed unrelated fact %v: %v (%v)", fact, found, e)
		}
	}
	if found, e := target.ContainsFact([]ast.ASTHandle{one, three}); e != nil || found {
		t.Fatalf("negation kept excluded fact: %v (%v)", found, e)
	}
	if _, err = target.FilterNegation(negative, []int{0}, []int{2}); err == nil {
		t.Fatal("out-of-range negation column accepted")
	}
}
