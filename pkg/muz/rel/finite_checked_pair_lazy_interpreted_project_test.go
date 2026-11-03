package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteCheckedPairLazyInterpretedProjectLifecycleAndMismatch(t *testing.T) {
	f := newTableRelationFixture(t)
	lazy := NewLazySparseFiniteFactory()
	theory := tableRelationTheoryRewriter(t, f.ast)
	condition, err := f.ast.MkVar(0, f.boolean)
	astTestMust(t, err)
	astTestPin(t, f.ast, condition)
	columns := map[uint32]int{0: 0}
	sorts := map[uint32]ast.ASTHandle{0: f.boolean}

	source := newCheckedLazyInterpretedPair(t, f, lazy, Fact{0}, Fact{1})
	projected, err := source.FilterInterpretedAndProject(f.relations, []ast.ASTHandle{f.boolean}, theory, []ast.ASTHandle{condition}, columns, sorts, []int{0})
	astTestMust(t, err)
	if rows, e := source.Facts(); e != nil || len(rows) != 2 {
		t.Fatal("checked fused transform mutated source", rows, e)
	}
	astTestMust(t, source.Close())
	if found, e := projected.Contains(Fact{}); e != nil || !found {
		t.Fatal("checked fused nullary result after source close", found, e)
	}
	astTestMust(t, projected.Close())

	corrupt := newCheckedLazyInterpretedPair(t, f, lazy, Fact{0})
	if inserted, e := corrupt.primary.(*LazySparseTable).Add(Fact{1}); e != nil || !inserted {
		t.Fatal("checked fused corruption setup", inserted, e)
	}
	if result, e := corrupt.FilterInterpretedAndProject(f.relations, []ast.ASTHandle{f.boolean}, theory, []ast.ASTHandle{condition}, columns, sorts, []int{0}); result != nil || !errors.Is(e, ErrCheckTable) {
		t.Fatal("checked fused transform accepted mismatched source", result, e)
	}
	if removed, e := corrupt.primary.(*LazySparseTable).Remove(Fact{1}); e != nil || !removed {
		t.Fatal("checked fused corruption repair", removed, e)
	}
	astTestMust(t, errors.Join(corrupt.Close(), lazy.Close()))
}
