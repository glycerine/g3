package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestExternalHeadAssemblyPreservesWorkingRelationAfterNativeCloseRefusal(t *testing.T) {
	f := newTableRelationFixture(t)
	poisoned := false
	operationCause := errors.New("external head project failed")
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return cause
			}
			switch kind {
			case ast.DatalogOpEmpty:
				return nil
			case ast.DatalogOpClone:
				if len(arguments) != 1 || len(outputs) != 1 {
					t.Fatal("clone callback shape", arguments, outputs)
				}
				return nil
			default:
				return ErrCompilerUnsupported
			}
		},
		Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			if kind != ast.DatalogOpProject || len(arguments) != 1 {
				t.Fatal("project callback shape", kind, arguments)
			}
			// The project application still pins its argument. Consume the
			// working relation's independent owner and fail the callback before
			// it trails the argument. Releasing the application consumes its last
			// native reference, so the preserved Go owner must remain retryable.
			if _, cause = f.ast.DecRefConsumed(arguments[0]); cause != nil {
				return ast.ASTHandle{}, cause
			}
			poisoned = true
			return ast.ASTHandle{}, operationCause
		},
	})
	astTestMust(t, err)
	defer func() { astTestMust(t, plugin.Close()) }()
	source, err := plugin.NewEmpty([]ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()

	result, failure := assembleExternalRuleHead(source, []externalHeadColumn{externalSourceHeadColumn(0)}, nil)
	if !poisoned || result != nil || !errors.Is(failure, operationCause) {
		t.Fatal("head assembly did not reach native working-relation refusal", poisoned, result, failure)
	}
	var receipt memory.ConstructionFailure
	var owned *externalRuleExecutionCleanupFailure
	if !errors.As(failure, &receipt) || !errors.As(failure, &owned) || receipt.RuntimeOwner() != f.ast.NumeralService().RuntimeOwner() {
		t.Fatal("head assembly refusal lost aggregate working owner", failure)
	}
	if len(owned.owner.relations) != 1 || owned.owner.relations[0] == nil || plugin.users != 2 {
		t.Fatal("head assembly cleanup changed exact working frontier", owned.owner.relations, plugin.users)
	}
	if _, err = source.Relation(); err != nil {
		t.Fatal("head assembly cleanup damaged borrowed source", err)
	}
	shown, exposed := receipt.Error(), receipt.Unwrap()
	// Repair only the deliberately consumed owner; the retry must finish the
	// preserved relation without revisiting the already closed project output.
	owned.owner.relations[0].relation = ast.ASTRef{}
	astTestMust(t, receipt.Cleanup())
	astTestMust(t, receipt.Cleanup())
	if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil || len(owned.owner.relations) != 0 || plugin.users != 1 {
		t.Fatal("head assembly retry changed receipt or ownership frontier", receipt.CleanupCause(), plugin.users)
	}
}
