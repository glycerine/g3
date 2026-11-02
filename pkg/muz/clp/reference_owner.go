package clp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type clpRefOwner struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

type clpRefCleanupFailure struct {
	cause      error
	cleanupErr error
	owner      *clpRefOwner
}

func (f *clpRefCleanupFailure) Error() string { return f.cause.Error() }
func (f *clpRefCleanupFailure) Unwrap() error { return f.cause }
func (f *clpRefCleanupFailure) Cleanup() error {
	f.cleanupErr = f.owner.close()
	return f.cleanupErr
}
func (f *clpRefCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *clpRefCleanupFailure) OriginalCause() error { return f.cause }
func (f *clpRefCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (o *clpRefOwner) take(ref ast.ASTRef) {
	o.refs = append(o.refs, ref)
}

func (o *clpRefOwner) close() error {
	for i := len(o.refs) - 1; i >= 0; i-- {
		if o.refs[i] == (ast.ASTRef{}) {
			continue
		}
		if err := o.manager.DelRef(&o.refs[i]); err != nil {
			return err
		}
	}
	o.refs = nil
	return nil
}

func (o *clpRefOwner) dispose() error {
	if err := o.close(); err != nil {
		return &clpRefCleanupFailure{cause: err, cleanupErr: err, owner: o}
	}
	return nil
}
