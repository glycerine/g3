package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestExternalPredicatePublicationPreservesUnpublishedRelationAfterNativeRefusal(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("external-predicate-publication-cleanup")
	astTestMust(t, err)
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	predicateRef, err := f.ast.NewRef(predicate)
	astTestMust(t, err)

	poisoned := false
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{ReduceAssign: func(declaration ast.ASTHandle, _, outputs []ast.ASTHandle) error {
		kind, cause := f.ast.GetDeclKind(declaration)
		if cause != nil || kind != ast.DatalogOpEmpty {
			return cause
		}
		if len(outputs) != 1 {
			t.Fatal("empty callback output shape", outputs)
		}
		// PredicateRelation has already read the declaration signature. Consume
		// both unpublished owners so predicate pinning and relation rollback
		// exercise their actual native refusal paths after NewEmpty returns.
		if cause = f.ast.DelRef(&predicateRef); cause != nil {
			return cause
		}
		if _, cause = f.ast.DecRefConsumed(outputs[0]); cause != nil {
			return cause
		}
		poisoned = true
		return nil
	}})
	astTestMust(t, err)

	relation, failure := plugin.PredicateRelation(predicate)
	if !poisoned || relation != nil || len(plugin.predicates) != 0 {
		t.Fatal("failed predicate publication exposed relation", poisoned, relation, len(plugin.predicates), failure)
	}
	var receipt memory.ConstructionFailure
	var owned *externalRelationConstructionFailure
	if !errors.As(failure, &receipt) || !errors.As(failure, &owned) || owned.owner.result == nil || receipt.RuntimeOwner() != f.ast.NumeralService().RuntimeOwner() {
		t.Fatal("predicate publication refusal lost unpublished relation owner", failure)
	}
	if plugin.users != 1 || owned.owner.result.plugin != plugin || owned.owner.result.relation == (ast.ASTRef{}) {
		t.Fatal("predicate publication receipt changed concrete owner", plugin.users, owned.owner.result)
	}
	shown, exposed := receipt.Error(), receipt.Unwrap()
	owned.owner.result.relation = ast.ASTRef{}
	astTestMust(t, receipt.Cleanup())
	astTestMust(t, receipt.Cleanup())
	if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil || owned.owner.result != nil || plugin.users != 0 {
		t.Fatal("predicate publication retry changed receipt or retained relation", receipt.CleanupCause(), plugin.users)
	}
	astTestMust(t, plugin.Close())
}
