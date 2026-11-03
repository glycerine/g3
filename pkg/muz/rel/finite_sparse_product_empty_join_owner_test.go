package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteSparseProductJoinStagesEmptyInnerUntilGarbageCollection(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-product-empty-join-owner")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 4)
	astTestMust(t, err)
	value := func(number uint64) ast.ASTHandle {
		handle, cause := f.ast.MkDatalogNumeral(number, sort)
		astTestMust(t, cause)
		astTestPin(t, f.ast, handle)
		return handle
	}
	zero, one := value(0), value(1)
	create := func(outer, inner ast.ASTHandle) *FiniteSparseProduct {
		product, cause := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
		astTestMust(t, cause)
		astTestMust(t, product.AddFact([]ast.ASTHandle{outer, inner}))
		t.Cleanup(func() { astTestMust(t, product.Close()) })
		return product
	}
	left, right := create(zero, zero), create(one, one)
	joined, err := left.Join(right, []int{1}, []int{1})
	astTestMust(t, err)
	defer func() { astTestMust(t, joined.Close()) }()

	if rows, cause := joined.SizeEstimateRows(); cause != nil || rows != 1 {
		t.Fatalf("empty inner join outer rows=%d err=%v, want staged row", rows, cause)
	}
	if empty, cause := joined.Empty(); cause != nil || !empty {
		t.Fatal("empty inner join did not collect as empty", empty, cause)
	}
	if rows, cause := joined.SizeEstimateRows(); cause != nil || rows != 0 {
		t.Fatalf("garbage-collected outer rows=%d err=%v", rows, cause)
	}
}
