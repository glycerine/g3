package spacer

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

// spacerAssertOwner is a caller's expr_ref held across an assert_expr, whose
// rewrite releases the solver's own copy of the input. Its release can run a
// native destructor that fails after consuming the root; the returned
// receipt resumes that release and any nested solver cleanup.
type spacerAssertOwner struct {
	manager *ast.ASTManager
	fml     ast.ASTRef
	nested  memory.ConstructionFailure
}

func newSpacerAssertOwner(manager *ast.ASTManager, formula ast.ASTHandle) (*spacerAssertOwner, error) {
	ref, err := manager.NewRef(formula)
	if err != nil {
		return nil, err
	}
	return &spacerAssertOwner{manager: manager, fml: ref}, nil
}

func (o *spacerAssertOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.fml != (ast.ASTRef{}) {
		return o.manager.DelRef(&o.fml)
	}
	return nil
}

// finish releases the owner after the assertion and reports cause; a
// release that leaves pending work returns a resumable receipt.
func (o *spacerAssertOwner) finish(cause error) error {
	if cause != nil {
		errors.As(cause, &o.nested)
	}
	cleanup := o.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	exposed := errors.Join(cause, cleanup)
	return &spacerAssertFailure{
		owner: o, cause: original, cleanup: cleanup, exposed: exposed, message: exposed.Error(),
	}
}

type spacerAssertFailure struct {
	owner                   *spacerAssertOwner
	cause, cleanup, exposed error
	message                 string
}

func (f *spacerAssertFailure) Error() string { return f.message }
func (f *spacerAssertFailure) Unwrap() error { return f.exposed }
func (f *spacerAssertFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}
func (f *spacerAssertFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *spacerAssertFailure) OriginalCause() error { return f.cause }
func (f *spacerAssertFailure) CleanupCause() error  { return f.cleanup }

var _ memory.ConstructionFailure = (*spacerAssertFailure)(nil)
