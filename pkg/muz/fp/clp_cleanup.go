package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/util/memory"
)

type clpBackendAttempt struct {
	manager      *ast.ASTManager
	preprocessed *hornPreprocessResult
	engine       *clp.Engine
	nested       memory.ConstructionFailure
}

type clpBackendCleanupFailure struct {
	cause, cleanupErr, exposed error
	message                    string
	attempt                    *clpBackendAttempt
}

func (f *clpBackendCleanupFailure) Error() string { return f.message }
func (f *clpBackendCleanupFailure) Unwrap() error { return f.exposed }
func (f *clpBackendCleanupFailure) Cleanup() error {
	f.cleanupErr = f.attempt.close()
	return f.cleanupErr
}
func (f *clpBackendCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *clpBackendCleanupFailure) OriginalCause() error { return f.cause }
func (f *clpBackendCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (a *clpBackendAttempt) close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.engine != nil {
		if err := a.engine.Close(); err != nil {
			return err
		}
		a.engine = nil
	}
	if a.preprocessed != nil {
		if err := a.preprocessed.Close(); err != nil {
			return err
		}
		a.preprocessed = nil
	}
	return nil
}

func (a *clpBackendAttempt) dispose() error {
	return a.finish(nil)
}

func (a *clpBackendAttempt) finish(cause error) error {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			a.nested = nested
		}
	}
	cleanup := a.close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
	}
	exposed := errors.Join(cause, cleanup)
	return &clpBackendCleanupFailure{
		cause: cause, cleanupErr: cleanup, exposed: exposed,
		message: exposed.Error(), attempt: a,
	}
}

var _ memory.ConstructionFailure = (*clpBackendCleanupFailure)(nil)
