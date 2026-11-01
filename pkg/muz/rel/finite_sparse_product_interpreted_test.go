package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteSparseProductInterpretedFilterSourceCondition(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-interpreted-byte")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 256)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	seven, nine := value(7), value(9)
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort, sort, sort}, []bool{true, true, false, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact([]ast.ASTHandle{seven, seven, nine, seven}))
	astTestMust(t, product.AddFact([]ast.ASTHandle{seven, nine, nine, seven}))
	variable := func(index uint32) ast.ASTHandle {
		h, e := f.ast.MkVar(index, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	firstEquality, err := f.ast.MkEq(variable(1), variable(2))
	astTestMust(t, err)
	astTestPin(t, f.ast, firstEquality)
	firstInequality, err := f.ast.MkNot(firstEquality)
	astTestMust(t, err)
	astTestPin(t, f.ast, firstInequality)
	secondEquality, err := f.ast.MkEq(variable(3), variable(2))
	astTestMust(t, err)
	astTestPin(t, f.ast, secondEquality)
	secondInequality, err := f.ast.MkNot(secondEquality)
	astTestMust(t, err)
	astTestPin(t, f.ast, secondInequality)
	condition, err := f.ast.MkAnd(firstInequality, secondInequality)
	astTestMust(t, err)
	astTestPin(t, f.ast, condition)
	if err = product.FilterInterpreted(condition, tableRelationTheoryRewriter(t, f.ast)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		fact []ast.ASTHandle
		want bool
	}{{[]ast.ASTHandle{seven, seven, nine, seven}, true}, {[]ast.ASTHandle{seven, nine, nine, seven}, false}} {
		found, e := product.ContainsFact(tc.fact)
		if e != nil || found != tc.want {
			t.Fatalf("interpreted filter row %v: got %v want %v (%v)", tc.fact, found, tc.want, e)
		}
	}
}

func TestFiniteSparseProductResetRecyclesIndexes(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-reset-domain")
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
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact([]ast.ASTHandle{zero, one}))
	astTestMust(t, product.AddFact([]ast.ASTHandle{two, one}))
	if err = product.Reset(); err != nil {
		t.Fatal(err)
	}
	if empty, e := product.Empty(); e != nil || !empty {
		t.Fatalf("reset relation not empty: %v (%v)", empty, e)
	}
	if len(product.inner) != 0 || len(product.available) != 0 {
		t.Fatalf("reset kept dead relation indexes: inner=%d available=%d", len(product.inner), len(product.available))
	}
	astTestMust(t, product.AddFact([]ast.ASTHandle{zero, two}))
	if len(product.inner) != 1 || len(product.available) != 0 {
		t.Fatalf("reset did not rebuild relation indexes: inner=%d available=%d", len(product.inner), len(product.available))
	}
}
