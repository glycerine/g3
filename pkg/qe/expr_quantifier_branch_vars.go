package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::add_var/get_branch_id.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type exprQEBranchVariable struct {
	variable, branch ast.ASTRef
	contains         *ast.ContainsApp
}
type exprQEBranchVariables struct {
	manager *ast.ASTManager
	entries []exprQEBranchVariable
	index   map[ast.ASTHandle]int
	pending []ast.ASTRef
	// order is the owner's m_var2contains table, which fixes the order
	// reset deallocates the contains_app objects in; nil keeps insertion
	// order.
	order *ast.ObjMap
}

func newExprQEBranchVariables(m *ast.ASTManager) (*exprQEBranchVariables, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	return &exprQEBranchVariables{manager: m, index: make(map[ast.ASTHandle]int)}, nil
}
func (s *exprQEBranchVariables) add(variable ast.ASTHandle) (err error) {
	if s == nil || s.manager == nil {
		return ast.ErrASTHandle
	}
	m := s.manager
	ref, err := m.NewRef(variable)
	if err != nil {
		return err
	}
	s.pending = append(s.pending, ref)
	if _, ok := s.index[variable]; ok {
		return nil
	}
	contains, err := ast.NewContainsApp(m, variable)
	if err != nil {
		return err
	}
	owned, err := m.NewRef(variable)
	if err != nil {
		return errors.Join(err, contains.Close())
	}
	s.entries = append(s.entries, exprQEBranchVariable{variable: owned, contains: contains})
	entry := &s.entries[len(s.entries)-1]
	varSort, err := m.GetSort(variable)
	if err != nil {
		return err
	}
	boolSort, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	isBV, err := m.IsBitVectorSort(varSort)
	if err != nil {
		return err
	}
	branch := variable
	if varSort != boolSort && !isBV {
		bvSort, e := m.BitVectorSort(20)
		if e != nil {
			return e
		}
		branch, e = m.MkFreshConst("b", bvSort)
		if e != nil {
			return e
		}
	}
	entry.branch, err = m.NewRef(branch)
	if err != nil {
		return err
	}
	s.index[variable] = len(s.entries) - 1
	if s.order != nil {
		return s.order.Insert(variable, [2]ast.ASTHandle{})
	}
	return nil
}

// release is the variable half of quant_elim_plugin::reset: m_trail (the
// fresh branch constants), then the contains_app objects of
// m_var2contains in table order. m_new_vars (pending) is left to Close.
func (s *exprQEBranchVariables) release() error {
	if s == nil || s.manager == nil {
		return nil
	}
	m := s.manager
	for i := range s.entries {
		if s.entries[i].branch != (ast.ASTRef{}) {
			if err := m.DelRef(&s.entries[i].branch); err != nil {
				return err
			}
		}
	}
	order := make([]int, 0, len(s.entries))
	if s.order != nil {
		for _, entry := range s.order.Entries() {
			if i, ok := s.index[entry.Key]; ok {
				order = append(order, i)
			}
		}
		s.order.Reset()
	}
	if len(order) != len(s.entries) {
		order = order[:0]
		for i := range s.entries {
			order = append(order, i)
		}
	}
	for _, i := range order {
		e := &s.entries[i]
		if e.contains != nil {
			if err := e.contains.Close(); err != nil {
				return err
			}
			e.contains = nil
		}
		if e.variable != (ast.ASTRef{}) {
			if err := m.DelRef(&e.variable); err != nil {
				return err
			}
		}
	}
	return nil
}

// releasePending is m_new_vars.reset(): forward.
func (s *exprQEBranchVariables) releasePending() error {
	if s == nil || s.manager == nil {
		return nil
	}
	for i := range s.pending {
		if s.pending[i] != (ast.ASTRef{}) {
			if err := s.manager.DelRef(&s.pending[i]); err != nil {
				return err
			}
		}
	}
	s.pending = s.pending[:0]
	return nil
}
func (s *exprQEBranchVariables) branch(variable ast.ASTHandle) (ast.ASTHandle, error) {
	if s == nil || s.manager == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	i, found := s.index[variable]
	if !found {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return s.manager.RefGet(s.entries[i].branch)
}
func (s *exprQEBranchVariables) consume(node *exprQETree) error {
	if s == nil || s.manager == nil || node == nil || node.manager != s.manager {
		return ast.ErrASTHandle
	}
	values := make([]ast.ASTHandle, len(s.pending))
	for i, ref := range s.pending {
		h, err := s.manager.RefGet(ref)
		if err != nil {
			return err
		}
		values[i] = h
	}
	if err := node.consumeVariables(values); err != nil {
		return err
	}
	for len(s.pending) > 0 {
		i := len(s.pending) - 1
		if err := s.manager.DelRef(&s.pending[i]); err != nil {
			return err
		}
		s.pending = s.pending[:i]
	}
	return nil
}
func (s *exprQEBranchVariables) Close() error {
	if s == nil || s.manager == nil {
		return nil
	}
	for len(s.pending) > 0 {
		i := len(s.pending) - 1
		if err := s.manager.DelRef(&s.pending[i]); err != nil {
			return err
		}
		s.pending = s.pending[:i]
	}
	for len(s.entries) > 0 {
		i := len(s.entries) - 1
		e := &s.entries[i]
		if e.contains != nil {
			if err := e.contains.Close(); err != nil {
				return err
			}
			e.contains = nil
		}
		for _, ref := range []*ast.ASTRef{&e.branch, &e.variable} {
			if *ref != (ast.ASTRef{}) {
				if err := s.manager.DelRef(ref); err != nil {
					return err
				}
			}
		}
		s.entries = s.entries[:i]
	}
	s.index = nil
	s.manager = nil
	return nil
}
