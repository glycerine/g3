package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func newCheckedLazyInterpretedPair(t *testing.T, f *tableRelationFixture, lazy *LazySparseFiniteFactory, facts ...Fact) *FiniteCheckedPair {
	t.Helper()
	signature, err := NewSignature([]uint64{2})
	astTestMust(t, err)
	factory, err := NewFiniteCheckedFactoryWithLazy("lazy_sparse", "sparse", lazy)
	astTestMust(t, err)
	pair, err := factory.New(signature)
	astTestMust(t, err)
	for _, fact := range facts {
		_, err = pair.Add(fact)
		astTestMust(t, err)
	}
	return pair
}

func TestFiniteCheckedPairLazyInterpretedFilterAndMismatch(t *testing.T) {
	f := newTableRelationFixture(t)
	lazy := NewLazySparseFiniteFactory()
	theory := tableRelationTheoryRewriter(t, f.ast)
	condition, err := f.ast.MkVar(0, f.boolean)
	astTestMust(t, err)
	astTestPin(t, f.ast, condition)
	columns := map[uint32]int{0: 0}
	sorts := map[uint32]ast.ASTHandle{0: f.boolean}

	pair := newCheckedLazyInterpretedPair(t, f, lazy, Fact{0}, Fact{1})
	if err = pair.FilterInterpreted(f.relations, []ast.ASTHandle{f.boolean}, theory, []ast.ASTHandle{condition}, columns, sorts); err != nil {
		t.Fatal("checked lazy interpreted filter", err)
	}
	rows, err := pair.Facts()
	if err != nil || len(rows) != 1 || len(rows[0]) != 1 || rows[0][0] != 1 {
		t.Fatal("checked lazy interpreted rows", rows, err)
	}
	astTestMust(t, pair.Close())

	corrupt := newCheckedLazyInterpretedPair(t, f, lazy, Fact{0})
	if inserted, e := corrupt.primary.(*LazySparseTable).Add(Fact{1}); e != nil || !inserted {
		t.Fatal("lazy corruption setup", inserted, e)
	}
	if err = corrupt.FilterInterpreted(f.relations, []ast.ASTHandle{f.boolean}, theory, []ast.ASTHandle{condition}, columns, sorts); !errors.Is(err, ErrCheckTable) {
		t.Fatal("checked lazy interpreted mismatch was not detected", err)
	}
	// Restore equality so both concrete owners can be retired normally.
	if removed, e := corrupt.primary.(*LazySparseTable).Remove(Fact{1}); e != nil || !removed {
		t.Fatal("lazy corruption repair", removed, e)
	}
	astTestMust(t, corrupt.Close())
	astTestMust(t, lazy.Close())
}
