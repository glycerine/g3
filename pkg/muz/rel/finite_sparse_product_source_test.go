package rel

// Source: src/test/dl_product_relation.cpp::test_finite_product_relation.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteSparseProductSourceDriver(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-source-byte")
	astTestMust(t, err)
	byteSort, err := f.ast.MkDatalogFiniteSort(name, 256)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, byteSort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	seven, nine := value(7), value(9)
	product := func(sorts int, mask []bool) *FiniteSparseProduct {
		columns := make([]ast.ASTHandle, sorts)
		for i := range columns {
			columns[i] = byteSort
		}
		p, e := NewFiniteSparseProduct(f.relations, columns, mask)
		astTestMust(t, e)
		t.Cleanup(func() { astTestMust(t, p.Close()) })
		return p
	}
	add := func(p *FiniteSparseProduct, values ...ast.ASTHandle) {
		t.Helper()
		astTestMust(t, p.AddFact(values))
	}
	contains := func(p *FiniteSparseProduct, values []ast.ASTHandle) bool {
		t.Helper()
		found, e := p.ContainsFact(values)
		astTestMust(t, e)
		return found
	}
	assertContains := func(p *FiniteSparseProduct, values []ast.ASTHandle, want bool) {
		t.Helper()
		if got := contains(p, values); got != want {
			t.Fatalf("contains %v = %v, want %v", values, got, want)
		}
	}

	// The C++ driver's table/table, table/inner, and inner/inner joins.
	r1, r2 := product(2, []bool{true, false}), product(2, []bool{true, false})
	add(r1, seven, seven)
	add(r1, seven, nine)
	add(r1, nine, seven)
	add(r2, nine, seven)
	add(r2, nine, nine)
	for _, tc := range []struct {
		left, right int
		want        bool
	}{{0, 0, false}, {0, 1, true}, {1, 1, true}} {
		joined, e := r1.Join(r2, []int{tc.left}, []int{tc.right})
		astTestMust(t, e)
		assertContains(joined, []ast.ASTHandle{seven, seven, nine, seven}, tc.want)
		astTestMust(t, joined.Close())
	}

	// Projection over inner columns, all inner columns, and the table column.
	r31 := product(3, []bool{true, false, false})
	f779 := []ast.ASTHandle{seven, seven, nine}
	f977 := []ast.ASTHandle{nine, seven, seven}
	f799 := []ast.ASTHandle{seven, nine, nine}
	add(r31, f779...)
	add(r31, f977...)
	add(r31, f799...)
	projected, e := r31.Project([]int{1})
	astTestMust(t, e)
	assertContains(projected, []ast.ASTHandle{seven, nine}, true)
	assertContains(projected, []ast.ASTHandle{nine, seven}, true)
	assertContains(projected, []ast.ASTHandle{seven, seven}, false)
	astTestMust(t, projected.Close())
	projected, e = r31.Project([]int{1, 2})
	astTestMust(t, e)
	assertContains(projected, []ast.ASTHandle{seven}, true)
	assertContains(projected, []ast.ASTHandle{nine}, true)
	astTestMust(t, projected.Close())
	projected, e = r31.Project([]int{0})
	astTestMust(t, e)
	assertContains(projected, []ast.ASTHandle{seven, nine}, true)
	assertContains(projected, []ast.ASTHandle{nine, seven}, false)
	assertContains(projected, []ast.ASTHandle{seven, seven}, true)
	assertContains(projected, []ast.ASTHandle{nine, nine}, true)
	astTestMust(t, projected.Close())

	// The source conjunction of two disequalities, evaluated after binding all
	// four finite relation columns.
	r41 := product(4, []bool{true, true, false, false})
	f7797 := []ast.ASTHandle{seven, seven, nine, seven}
	f7997 := []ast.ASTHandle{seven, nine, nine, seven}
	add(r41, f7797...)
	add(r41, f7997...)
	variable := func(index uint32) ast.ASTHandle {
		h, e := f.ast.MkVar(index, byteSort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	eq12, err := f.ast.MkEq(variable(1), variable(2))
	astTestMust(t, err)
	astTestPin(t, f.ast, eq12)
	notEq12, err := f.ast.MkNot(eq12)
	astTestMust(t, err)
	astTestPin(t, f.ast, notEq12)
	eq32, err := f.ast.MkEq(variable(3), variable(2))
	astTestMust(t, err)
	astTestPin(t, f.ast, eq32)
	notEq32, err := f.ast.MkNot(eq32)
	astTestMust(t, err)
	astTestPin(t, f.ast, notEq32)
	condition, err := f.ast.MkAnd(notEq12, notEq32)
	astTestMust(t, err)
	astTestPin(t, f.ast, condition)
	astTestMust(t, r41.FilterInterpreted(condition, tableRelationTheoryRewriter(t, f.ast)))
	assertContains(r41, f7797, true)
	assertContains(r41, f7997, false)

	// The source's repeated-column negation filter over a two-column relation.
	astTestMust(t, r31.Reset())
	for _, fact := range [][]ast.ASTHandle{f779, f977, f799} {
		add(r31, fact...)
	}
	astTestMust(t, r1.Reset())
	add(r1, seven, seven)
	add(r1, seven, nine)
	removed, err := r31.FilterNegation(r1, []int{1, 0, 1}, []int{0, 0, 1})
	astTestMust(t, err)
	if removed != 1 {
		t.Fatalf("source negation removed %d rows, want 1", removed)
	}
	assertContains(r31, f779, false)
	assertContains(r31, f977, true)
	assertContains(r31, f799, true)
}
