package rel

// Source: src/muz/rel/dl_external_relation.cpp external assignment
// declarations and src/api/api_datalog.cpp callback cleanup transfer.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalAssignmentAttempt struct {
	manager *ast.ASTManager
	nested  memory.ConstructionFailure
	roots   []ast.ASTRef
}

func (a *externalAssignmentAttempt) close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	for len(a.roots) != 0 {
		last := len(a.roots) - 1
		if a.roots[last] == (ast.ASTRef{}) {
			a.roots = a.roots[:last]
			continue
		}
		err := a.manager.DelRef(&a.roots[last])
		if a.roots[last] == (ast.ASTRef{}) {
			a.roots = a.roots[:last]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type externalAssignmentCleanupFailure struct {
	attempt                  *externalAssignmentAttempt
	cause, cleanup, reported error
	text                     string
}

var _ memory.ConstructionFailure = (*externalAssignmentCleanupFailure)(nil)

func (f *externalAssignmentCleanupFailure) Error() string        { return f.text }
func (f *externalAssignmentCleanupFailure) Unwrap() error        { return f.reported }
func (f *externalAssignmentCleanupFailure) OriginalCause() error { return f.cause }
func (f *externalAssignmentCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *externalAssignmentCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *externalAssignmentCleanupFailure) Cleanup() error {
	f.cleanup = f.attempt.close()
	return f.cleanup
}

func finishExternalAssignment(attempt *externalAssignmentAttempt, cause error) error {
	if cause != nil {
		errors.As(cause, &attempt.nested)
	}
	cleanup := attempt.close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
		return &externalAssignmentCleanupFailure{attempt: attempt, cause: cause, cleanup: cleanup, reported: cause, text: cause.Error()}
	}
	reported := errors.Join(cause, cleanup)
	return &externalAssignmentCleanupFailure{attempt: attempt, cause: cause, cleanup: cleanup, reported: reported, text: reported.Error()}
}
