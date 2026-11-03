package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestFormulaCheckedTableConstructionCleanupRefusalIsRetryable(t *testing.T) {
	f := newTableRelationFixture(t)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	checked, err := NewFormulaCheckedTable(table, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)

	// A consumed copy leaves the checked table's native AST root at a real
	// refused-release frontier. Constructor cleanup must return the owner.
	consumed := checked.formula
	astTestMust(t, f.ast.DelRef(&consumed))
	origin := errors.New("formula checked clone verification failed")
	failure := finishFormulaCheckedTableConstruction(origin, checked)
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) || receipt.RuntimeOwner() != f.ast.NumeralService().RuntimeOwner() {
		t.Fatal("formula checked cleanup refusal lost typed owner", failure)
	}
	if !errors.Is(receipt.OriginalCause(), origin) || receipt.CleanupCause() == nil {
		t.Fatal("formula checked receipt lost separate causes", receipt.OriginalCause(), receipt.CleanupCause())
	}
	owned, ok := receipt.(*formulaCheckedTableConstructionFailure)
	if !ok || owned.owner != checked || checked.table == nil {
		t.Fatal("formula checked receipt lost exact relation owner", owned)
	}
	shown, exposed := receipt.Error(), receipt.Unwrap()
	if err = receipt.Cleanup(); err == nil || receipt.Error() != shown || receipt.Unwrap() != exposed {
		t.Fatal("formula checked cleanup retry changed stable refusal", err)
	}
	checked.formula = ast.ASTRef{}
	astTestMust(t, receipt.Cleanup())
	astTestMust(t, receipt.Cleanup())
	if receipt.CleanupCause() != nil || owned.owner != nil || !checked.closed {
		t.Fatal("formula checked cleanup retry left owner", receipt.CleanupCause(), owned.owner, checked.closed)
	}
}
