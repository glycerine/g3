package smtcore

// smtStringScope mirrors the lifetime of a C++ theory_str function's local
// expr_ref / expr_ref_vector / th_rewriter objects (instantiate_axiom_Substr,
// instantiate_axiom_Replace). Owners are released in reverse declaration
// order at scope exit; an expr_ref_vector releases its elements forward
// (ref_vector_core destructor).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtStringScopeEntry struct {
	refs []ast.ASTRef
	hook func() error
}

type smtStringScope struct {
	theory  *SMTStringTheory
	entries []*smtStringScopeEntry
}

type smtStringScopeVector struct {
	scope  *smtStringScope
	entry  *smtStringScopeEntry
	values []ast.ASTHandle
}

func (s *smtStringScope) close() (result error) {
	m := s.theory.state.manager
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if e.hook != nil {
			result = errors.Join(result, e.hook())
		}
		for j := range e.refs {
			result = errors.Join(result, m.DelRef(&e.refs[j]))
		}
	}
	s.entries = nil
	return result
}

// own is `expr_ref x(value, m);`.
func (s *smtStringScope) own(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return value, err
	}
	ref, err := s.theory.state.manager.NewRef(value)
	if err != nil {
		return value, err
	}
	s.entries = append(s.entries, &smtStringScopeEntry{refs: []ast.ASTRef{ref}})
	return value, nil
}

// ownRef is `expr_ref x(value, m);` returning the reference, for owners
// that are later overwritten (a rewriter's in/out argument).
func (s *smtStringScope) ownRef(value ast.ASTHandle) (*ast.ASTRef, error) {
	ref, err := s.theory.state.manager.NewRef(value)
	if err != nil {
		return nil, err
	}
	e := &smtStringScopeEntry{refs: []ast.ASTRef{ref}}
	s.entries = append(s.entries, e)
	return &e.refs[0], nil
}

// hook registers a destructor (for example a local th_rewriter).
func (s *smtStringScope) hook(f func() error) {
	s.entries = append(s.entries, &smtStringScopeEntry{hook: f})
}

// vector is `expr_ref_vector v(m);`.
func (s *smtStringScope) vector() *smtStringScopeVector {
	e := &smtStringScopeEntry{}
	s.entries = append(s.entries, e)
	return &smtStringScopeVector{scope: s, entry: e}
}

func (v *smtStringScopeVector) push(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return value, err
	}
	ref, err := v.scope.theory.state.manager.NewRef(value)
	if err != nil {
		return value, err
	}
	v.entry.refs = append(v.entry.refs, ref)
	v.values = append(v.values, value)
	return value, nil
}

// rewriteOwned is `expr_ref x_rw(x, m); rw(x_rw); assert_axiom(x_rw);` with
// x_rw living until scope exit.
func (s *smtStringScope) rewriteOwned(expression ast.ASTHandle, rw interface {
	Rewrite(ast.ASTHandle, ast.ASTRef) error
}) error {
	m := s.theory.state.manager
	ref, err := s.ownRef(expression)
	if err != nil {
		return err
	}
	if err := rw.Rewrite(expression, *ref); err != nil {
		return err
	}
	value, err := m.RefGet(*ref)
	if err != nil {
		return err
	}
	return s.theory.assertAxiom(value)
}
