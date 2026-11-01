package spacer

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

// literalExpansion owns the source extra_lits vector and construction
// intermediates until the candidate checks or an interrupted expansion finish.
type literalExpansion struct {
	manager           *ast.ASTManager
	literals          []ast.ASTHandle
	refs              []ast.ASTHandle
	cause, cleanupErr error
}

func (e *literalExpansion) keep(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = e.manager.IncRef(h); err != nil {
		return ast.ASTHandle{}, err
	}
	e.refs = append(e.refs, h)
	return h, nil
}

func (e *literalExpansion) Close() (result error) {
	for len(e.refs) != 0 {
		i := len(e.refs) - 1
		consumed, err := e.manager.DecRefConsumed(e.refs[i])
		result = errors.Join(result, err)
		if !consumed {
			return result
		}
		e.refs = e.refs[:i]
	}
	e.literals = nil
	return result
}

func (e *literalExpansion) Error() string  { return e.cause.Error() }
func (e *literalExpansion) Unwrap() error  { return e.cause }
func (e *literalExpansion) Cleanup() error { e.cleanupErr = e.Close(); return e.cleanupErr }
func (e *literalExpansion) RuntimeOwner() *memory.Runtime {
	return e.manager.NumeralService().RuntimeOwner()
}
func (e *literalExpansion) OriginalCause() error { return e.cause }
func (e *literalExpansion) CleanupCause() error  { return e.cleanupErr }
