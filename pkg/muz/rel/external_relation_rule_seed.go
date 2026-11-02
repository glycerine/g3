package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
)

// externalRuleSeedOwner keeps the source nullary empty/total pair retryable
// across native cleanup refusal. Complement borrows empty, so both owners stay
// live through saturation and retire in reverse construction order.
type externalRuleSeedOwner struct {
	plugin       *ExternalRelationPlugin
	nested       memory.ConstructionFailure
	empty, total *ExternalRelation
}

func (o *externalRuleSeedOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.total != nil {
		if err := o.total.Close(); err != nil {
			return err
		}
		o.total = nil
	}
	if o.empty != nil {
		if err := o.empty.Close(); err != nil {
			return err
		}
		o.empty = nil
	}
	return nil
}

type externalRuleSeedCleanupFailure struct {
	owner                 *externalRuleSeedOwner
	cause, cleanup, shown error
	message               string
}

func (f *externalRuleSeedCleanupFailure) Error() string { return f.message }
func (f *externalRuleSeedCleanupFailure) Unwrap() error { return f.shown }
func (f *externalRuleSeedCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.plugin.ast.NumeralService().RuntimeOwner()
}
func (f *externalRuleSeedCleanupFailure) OriginalCause() error { return f.cause }
func (f *externalRuleSeedCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *externalRuleSeedCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishExternalRuleSeed(owner *externalRuleSeedOwner, cause error) error {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			owner.nested = nested
		}
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
		return &externalRuleSeedCleanupFailure{owner: owner, cause: cause, cleanup: cleanup, shown: cause, message: cause.Error()}
	}
	shown := errors.Join(cause, cleanup)
	return &externalRuleSeedCleanupFailure{owner: owner, cause: cause, cleanup: cleanup, shown: shown, message: shown.Error()}
}

var _ memory.ConstructionFailure = (*externalRuleSeedCleanupFailure)(nil)
