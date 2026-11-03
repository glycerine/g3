package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalIntervalCleanupFrontier struct {
	runtime  *memory.Runtime
	cleanup  error
	refusals int
	cleanups int
}

func (f *externalIntervalCleanupFrontier) Error() string        { return ast.ErrASTPending.Error() }
func (f *externalIntervalCleanupFrontier) Unwrap() error        { return ast.ErrASTPending }
func (f *externalIntervalCleanupFrontier) OriginalCause() error { return ast.ErrASTPending }
func (f *externalIntervalCleanupFrontier) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalCleanupFrontier) RuntimeOwner() *memory.Runtime {
	return f.runtime
}
func (f *externalIntervalCleanupFrontier) Cleanup() error {
	f.cleanups++
	if f.refusals != 0 {
		f.refusals--
		f.cleanup = ast.ErrASTPending
		return f.cleanup
	}
	f.cleanup = nil
	return nil
}

var _ memory.ConstructionFailure = (*externalIntervalCleanupFrontier)(nil)

func TestExternalIntervalProductCleanupOnlyCommitPublishesWithoutReplay(t *testing.T) {
	f := newTableRelationFixture(t)
	runtime := f.ast.NumeralService().RuntimeOwner()
	for _, test := range []struct {
		name    string
		wrapped bool
	}{
		{name: "target-without-delta"},
		{name: "target-with-delta", wrapped: true},
		{name: "delta", wrapped: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			callbackCount := 0
			refusals := 1
			if test.wrapped {
				// ExternalRelation's finalizer refuses first; the product owner
				// then refuses while adopting that pending receipt.
				refusals = 2
			}
			frontier := &externalIntervalCleanupFrontier{runtime: runtime, refusals: refusals}
			original := NewIntervalEmpty(1)
			candidate := NewIntervalEmpty(1)
			destination := original
			assignment := applyExternalIntervalAssignment(&destination, candidate, func() error {
				callbackCount++
				return finishExternalAssignment(&externalAssignmentAttempt{
					manager: f.ast,
					nested:  frontier,
				}, nil)
			})
			if assignment == nil || !externalAssignmentWasCommitted(assignment) || !ExternalAssignmentCommitted(assignment) {
				t.Fatal("successful external assignment lost cleanup-only commit marker", assignment)
			}
			operation := assignment
			if test.wrapped {
				operation = finishExternalIntervalUnion(&externalIntervalUnionOwner{runtime: runtime}, assignment)
				if operation == nil || !externalAssignmentWasCommitted(operation) || !ExternalAssignmentCommitted(operation) {
					t.Fatal("product cleanup owner lost assignment commit marker", operation)
				}
			}

			if destination != candidate {
				t.Fatal("committed external stage did not publish matching interval candidate")
			}
			var receipt memory.ConstructionFailure
			if !errors.As(operation, &receipt) || receipt.RuntimeOwner() != runtime {
				t.Fatal("cleanup-only commit lost retry owner", operation)
			}
			if err := receipt.Cleanup(); err != nil {
				t.Fatal(err)
			}
			if callbackCount != 1 || destination != candidate || frontier.cleanups != refusals+1 {
				t.Fatal("cleanup retry replayed or reverted committed assignment",
					callbackCount, destination == candidate, frontier.cleanups)
			}
			if err := receipt.Cleanup(); err != nil {
				t.Fatal(err)
			}
			if callbackCount != 1 || destination != candidate || frontier.cleanups != refusals+1 {
				t.Fatal("completed cleanup changed committed assignment",
					callbackCount, destination == candidate, frontier.cleanups)
			}
		})
	}
}

func TestExternalIntervalProductCallbackRefusalDoesNotPublishCandidate(t *testing.T) {
	original := NewIntervalEmpty(1)
	candidate := NewIntervalEmpty(1)
	destination := original
	callbackCount := 0
	refusal := errors.New("external callback refused")
	operation := applyExternalIntervalAssignment(&destination, candidate, func() error {
		callbackCount++
		return refusal
	})
	if operation != refusal || callbackCount != 1 || destination != original {
		t.Fatal("refused external callback published interval candidate", operation, callbackCount)
	}
}
