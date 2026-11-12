package qe

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// SolverNNFAdapter is the concrete retained i_solver_context relevance and
// atom-normalization adapter.
type SolverNNFAdapter struct {
	self    *SolverNNFAdapter
	context *SolverContext
	closed  bool
}

func NewSolverNNFAdapter(context *SolverContext) (*SolverNNFAdapter, error) {
	if context == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := context.Manager(); err != nil {
		return nil, err
	}
	a := &SolverNNFAdapter{context: context}
	a.self = a
	return a, nil
}

func (a *SolverNNFAdapter) open() error {
	if a == nil || a.self != a || a.context == nil {
		return ast.ErrASTHandle
	}
	if a.closed {
		return arena.ErrClosed
	}
	_, err := a.context.Manager()
	return err
}

// IsRelevant is i_solver_context::is_relevant::operator(): it asks the
// context's own contains_app for each current variable (m_s.contains(i)),
// so the check_pred caches are the ones quant_elim_plugin::reset releases.
func (a *SolverNNFAdapter) IsRelevant(expression ast.ASTHandle) (bool, error) {
	if err := a.open(); err != nil {
		return false, err
	}
	count, err := a.context.NumVariables()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		contains, err := a.context.Contains(i)
		if err != nil {
			return false, err
		}
		found, err := contains.Check(expression)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

func (a *SolverNNFAdapter) NormalizeAtom(expression ast.ASTHandle, positive bool) (ast.ASTHandle, error) {
	if err := a.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return a.context.MkAtom(expression, positive)
}

// NormalizeAtomRef is mk_atom_fn: the result expr_ref passes to the caller.
func (a *SolverNNFAdapter) NormalizeAtomRef(expression ast.ASTHandle, positive bool) (ast.ASTRef, error) {
	if err := a.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return a.context.mkAtomRef(expression, positive)
}

func (a *SolverNNFAdapter) Close() error {
	if a == nil || a.self != a {
		return ast.ErrASTHandle
	}
	a.closed = true
	return nil
}
