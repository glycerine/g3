package fp

// Source: dl_relation_manager.cpp default_relation_select_equal_and_project_fn.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

// The source scoped clone remains owned until projection succeeds. Neither
// temporary is exposed to the caller while clone cleanup is unfinished.
type externalIntervalUnarySelectAttempt struct {
	working   *rel.ExternalIntervalProduct
	projected *rel.ExternalIntervalProduct
	nested    memory.ConstructionFailure
	runtime   *memory.Runtime
}

func (a *externalIntervalUnarySelectAttempt) close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.working != nil {
		if err := a.working.Close(); err != nil {
			return err
		}
		a.working = nil
	}
	if a.projected != nil {
		if err := a.projected.Close(); err != nil {
			return err
		}
		a.projected = nil
	}
	return nil
}

type externalIntervalUnarySelectFailure struct {
	original, cleanup, reported error
	text                        string
	owner                       *externalIntervalUnarySelectAttempt
}

var _ memory.ConstructionFailure = (*externalIntervalUnarySelectFailure)(nil)

func (f *externalIntervalUnarySelectFailure) Error() string        { return f.text }
func (f *externalIntervalUnarySelectFailure) Unwrap() error        { return f.reported }
func (f *externalIntervalUnarySelectFailure) OriginalCause() error { return f.original }
func (f *externalIntervalUnarySelectFailure) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalUnarySelectFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *externalIntervalUnarySelectFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishExternalIntervalUnarySelectFailure(cause error, owner *externalIntervalUnarySelectAttempt) error {
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &externalIntervalUnarySelectFailure{
		original: original, cleanup: cleanup, reported: reported,
		text: reported.Error(), owner: owner,
	}
}

func (a *externalIntervalUnarySelectAttempt) complete() (*rel.ExternalIntervalProduct, error) {
	if a.working != nil {
		if err := a.working.Close(); err != nil {
			return nil, finishExternalIntervalUnarySelectFailure(err, a)
		}
		a.working = nil
	}
	result := a.projected
	a.projected = nil
	return result, nil
}

func selectExternalIntervalUnary(source *rel.ExternalIntervalProduct, column uint32, value ast.ASTHandle, runtime *memory.Runtime) (*rel.ExternalIntervalProduct, error) {
	owner := &externalIntervalUnarySelectAttempt{runtime: runtime}
	var err error
	owner.working, err = source.Clone()
	if err != nil {
		return nil, finishExternalIntervalUnarySelectFailure(err, owner)
	}
	if err = owner.working.FilterEqual(column, value); err != nil {
		return nil, finishExternalIntervalUnarySelectFailure(err, owner)
	}
	// The source fallback always projects, including an empty filtered clone.
	owner.projected, err = owner.working.Project([]uint32{column})
	if err != nil {
		return nil, finishExternalIntervalUnarySelectFailure(err, owner)
	}
	return owner.complete()
}
