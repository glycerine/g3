package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestSpacerModelValueCleanupReceiptKeepsNativeOwner(t *testing.T) {
	manager, _ := newSpacerTestServices(t)
	truth, err := manager.MkTrue()
	depthMust(t, err)
	value, err := manager.NewRef(truth)
	depthMust(t, err)
	stale := value
	depthMust(t, manager.DelRef(&value))
	owner := &spacerModelValueOwner{manager: manager, value: stale}
	origin := errors.New("spacer model Boolean probe failed")
	failure := finishSpacerModelValue(owner, origin)
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) || receipt.RuntimeOwner() != manager.NumeralService().RuntimeOwner() {
		t.Fatal("model-value native refusal lost typed owner", failure)
	}
	if receipt.OriginalCause() != origin || receipt.CleanupCause() == nil || !errors.Is(failure, origin) {
		t.Fatal("model-value receipt lost operation or cleanup cause", receipt.OriginalCause(), receipt.CleanupCause())
	}
	message, exposed := receipt.Error(), receipt.Unwrap()
	if err = receipt.Cleanup(); err == nil {
		t.Fatal("model-value retry unexpectedly consumed stale native owner")
	}
	owner.value = ast.ASTRef{}
	depthMust(t, receipt.Cleanup())
	depthMust(t, receipt.Cleanup())
	if receipt.CleanupCause() != nil || receipt.OriginalCause() != origin || receipt.Error() != message || receipt.Unwrap() != exposed {
		t.Fatal("model-value cleanup retry changed stable receipt identity")
	}
}
