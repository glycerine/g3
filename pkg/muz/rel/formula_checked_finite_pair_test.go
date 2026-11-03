package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
)

func newFormulaCheckedFinitePairFixture(t *testing.T, f *tableRelationFixture, facts ...Fact) *FormulaCheckedFinitePair {
	t.Helper()
	signature, err := NewSignature([]uint64{2})
	astTestMust(t, err)
	factory, err := NewFiniteCheckedFactory("sparse", "sparse")
	astTestMust(t, err)
	pair, err := factory.New(signature)
	astTestMust(t, err)
	for _, fact := range facts {
		_, err = pair.Add(fact)
		astTestMust(t, err)
	}
	result, err := NewFormulaCheckedFinitePair(f.relations, []ast.ASTHandle{f.boolean}, pair, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	return result
}

func TestFormulaCheckedFinitePairTypedOperationsAndOwnership(t *testing.T) {
	f := newTableRelationFixture(t)
	left := newFormulaCheckedFinitePairFixture(t, f, Fact{0})
	right := newFormulaCheckedFinitePairFixture(t, f, Fact{0}, Fact{1})
	joined, err := left.Join(right, []int{0}, []int{0})
	astTestMust(t, err)
	if rows, e := joined.Facts(); e != nil || len(rows) != 1 || len(rows[0]) != 2 {
		t.Fatal("joined nested checker", rows, e)
	}
	projected, err := joined.Project([]int{1})
	astTestMust(t, err)
	if found, e := projected.Contains(Fact{0}); e != nil || !found {
		t.Fatal("projected nested checker", found, e)
	}
	selected, err := right.SelectEqualAndProject(1, 0)
	astTestMust(t, err)
	if found, e := selected.Contains(Fact{}); e != nil || !found {
		t.Fatal("selected nested checker", found, e)
	}
	delta, err := projected.EmptyLike()
	astTestMust(t, err)
	target, err := projected.EmptyLike()
	astTestMust(t, err)
	astTestMust(t, target.UnionFrom(projected, delta))
	if found, e := delta.Contains(Fact{0}); e != nil || !found {
		t.Fatal("nested union delta", found, e)
	}
	primary, checker, formula, err := target.ConcreteKinds()
	if err != nil || primary != "sparse" || checker != "sparse" || !formula {
		t.Fatal("nested concrete kinds", primary, checker, formula, err)
	}
	clone, err := target.CloneFinite()
	astTestMust(t, err)
	astTestMust(t, errors.Join(left.Close(), right.Close(), joined.Close(), projected.Close(), selected.Close(), delta.Close(), target.Close()))
	if found, e := clone.Contains(Fact{0}); e != nil || !found {
		t.Fatal("clone did not survive sources", found, e)
	}
	astTestMust(t, clone.Close())
}

func TestFormulaCheckedFinitePairDetectsInnerCorruption(t *testing.T) {
	f := newTableRelationFixture(t)
	nested := newFormulaCheckedFinitePairFixture(t, f, Fact{0})
	if _, err := nested.pair.primary.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	if rows, err := nested.Facts(); !errors.Is(err, ErrCheckTable) || rows != nil {
		t.Fatal("inner corruption was not detected", rows, err)
	}
	if _, err := nested.pair.primary.(*SparseTable).Remove(Fact{1}); err != nil {
		t.Fatal(err)
	}
	astTestMust(t, nested.Close())
}

func TestFormulaCheckedFinitePairConstructorFailureLeavesPairOwned(t *testing.T) {
	f := newTableRelationFixture(t)
	signature, err := NewSignature([]uint64{2})
	astTestMust(t, err)
	factory, err := NewFiniteCheckedFactory("sparse", "sparse")
	astTestMust(t, err)
	pair, err := factory.New(signature)
	astTestMust(t, err)
	if result, e := NewFormulaCheckedFinitePair(f.relations, []ast.ASTHandle{f.boolean}, pair, smtcore.SMTSolverConfig{}); e == nil || result != nil {
		t.Fatal("invalid formula configuration constructed nested checker", result, e)
	}
	if inserted, e := pair.Add(Fact{1}); e != nil || !inserted {
		t.Fatal("constructor consumed pair on failure", inserted, e)
	}
	astTestMust(t, pair.Close())
}

func TestFormulaCheckedFinitePairRejectsMismatchedEmptySignature(t *testing.T) {
	f := newTableRelationFixture(t)
	signature, err := NewSignature([]uint64{3})
	astTestMust(t, err)
	factory, err := NewFiniteCheckedFactory("sparse", "sparse")
	astTestMust(t, err)
	pair, err := factory.New(signature)
	astTestMust(t, err)
	if result, e := NewFormulaCheckedFinitePair(f.relations, []ast.ASTHandle{f.boolean}, pair, formulaCheckerConfig(t, f.ast)); !errors.Is(e, ErrRelationVerification) || result != nil {
		t.Fatal("mismatched signature constructed nested checker", result, e)
	}
	if inserted, e := pair.Add(Fact{2}); e != nil || !inserted {
		t.Fatal("signature rejection consumed pair", inserted, e)
	}
	astTestMust(t, pair.Close())
}

func TestFormulaCheckedFinitePairLazySizeEstimate(t *testing.T) {
	f := newTableRelationFixture(t)
	signature, err := NewSignature([]uint64{2})
	astTestMust(t, err)
	lazy := NewLazySparseFiniteFactory()
	factory, err := NewFiniteCheckedFactoryWithLazy("lazy_sparse", "sparse", lazy)
	astTestMust(t, err)
	pair, err := factory.New(signature)
	astTestMust(t, err)
	_, err = pair.Add(Fact{1})
	astTestMust(t, err)
	nested, err := NewFormulaCheckedFinitePair(f.relations, []ast.ASTHandle{f.boolean}, pair, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	if size, e := nested.SizeEstimateBytes(); e != nil || size == 0 {
		t.Fatal("lazy nested byte estimate", size, e)
	}
	astTestMust(t, nested.Close())
	astTestMust(t, lazy.Close())
}

func TestFormulaCheckedFinitePairConstructionCleanupRefusalIsRetryable(t *testing.T) {
	f := newTableRelationFixture(t)
	signature, err := NewSignature([]uint64{2})
	astTestMust(t, err)
	factory, err := NewFiniteCheckedFactory("sparse", "sparse")
	astTestMust(t, err)
	pair, err := factory.New(signature)
	astTestMust(t, err)
	_, err = pair.Add(Fact{1})
	astTestMust(t, err)

	relation, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	formula, err := NewFormulaCheckedTable(relation, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	// Consume a copy of the owned root. The stored ref now deterministically
	// refuses native release, while the empty relation remains usable long
	// enough for construction verification to detect the row mismatch.
	consumed := formula.formula
	astTestMust(t, f.ast.DelRef(&consumed))

	result, failure := adoptFormulaCheckedFinitePair(pair, formula)
	var receipt memory.ConstructionFailure
	if result != nil || !errors.As(failure, &receipt) || receipt.RuntimeOwner() != f.ast.NumeralService().RuntimeOwner() {
		t.Fatal("nested construction cleanup refusal lost retry owner", result, failure)
	}
	if !errors.Is(receipt.OriginalCause(), ErrRelationVerification) || receipt.CleanupCause() == nil {
		t.Fatal("nested construction receipt lost separate causes", receipt.OriginalCause(), receipt.CleanupCause())
	}
	owned, ok := receipt.(*formulaCheckedFinitePairConstructionFailure)
	if !ok || owned.attempt.owner == nil || owned.attempt.owner.pair != nil || owned.attempt.owner.formula != formula {
		t.Fatal("nested construction receipt lost exact cleanup frontier", owned)
	}
	shown, exposed := receipt.Error(), receipt.Unwrap()
	if err = receipt.Cleanup(); err == nil || receipt.Error() != shown || receipt.Unwrap() != exposed {
		t.Fatal("nested construction retry changed stable refusal", err)
	}
	formula.formula = ast.ASTRef{}
	astTestMust(t, receipt.Cleanup())
	astTestMust(t, receipt.Cleanup())
	if receipt.CleanupCause() != nil || owned.attempt.owner != nil {
		t.Fatal("nested construction cleanup retry left owner", receipt.CleanupCause(), owned.attempt.owner)
	}
}
