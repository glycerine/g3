package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
)

// externalRuleSeedOwner keeps every full relation used to introduce rule
// columns retryable across native cleanup refusal. Totals retire in reverse
// construction order after any nested constructor receipt.
type externalRuleSeedOwner struct {
	plugin *ExternalRelationPlugin
	nested memory.ConstructionFailure
	totals []*ExternalRelation
}

func (o *externalRuleSeedOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	for index := len(o.totals) - 1; index >= 0; index-- {
		if o.totals[index] == nil {
			continue
		}
		if err := o.totals[index].Close(); err != nil {
			return err
		}
		o.totals[index] = nil
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
		errors.As(cause, &owner.nested)
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
