package smtcore

// Source: src/util/backtrackable_set.h:28-100; concrete qset/qsset uses in
// src/smt/smt_model_finder.cpp:2739-2864.

import "github.com/glycerine/g3/pkg/ast"

type quantifierSetTrail struct {
	value    ast.ASTHandle
	inserted bool
}

// scopedQuantifierSet is the concrete quantifier-pointer set used by the
// model-finder hint solver. Handles are borrowed; the AST owner retains them.
// onUndo is nil for m_residue and resets quantifier_info's chosen function
// for m_satisfied. The source invokes it before reversing each scoped edit.
// scopedQuantifierSet is backtrackable_set over an obj_hashtable: values
// keeps the source table's layout, and popping replays erase/insert.
type scopedQuantifierSet struct {
	values *ast.ObjMap
	trail  []quantifierSetTrail
	scopes []int
	onUndo func(ast.ASTHandle, bool)
}

func newScopedQuantifierSet(m *ast.ASTManager, onUndo func(ast.ASTHandle, bool)) *scopedQuantifierSet {
	return &scopedQuantifierSet{
		values: ast.NewObjMap(m),
		onUndo: onUndo,
	}
}

func (s *scopedQuantifierSet) contains(q ast.ASTHandle) bool {
	found, err := s.values.Contains(q)
	return err == nil && found
}

func (s *scopedQuantifierSet) empty() bool { return s.values.Len() == 0 }

func (s *scopedQuantifierSet) insert(q ast.ASTHandle) error {
	if s.contains(q) {
		return nil
	}
	if err := s.values.Insert(q, [2]ast.ASTHandle{}); err != nil {
		return err
	}
	if len(s.scopes) != 0 {
		s.trail = append(s.trail, quantifierSetTrail{value: q, inserted: true})
	}
	return nil
}

func (s *scopedQuantifierSet) erase(q ast.ASTHandle) error {
	if len(s.scopes) == 0 {
		// Source line 62 calls insert in the level-zero erase branch.
		return s.insert(q)
	}
	if !s.contains(q) {
		return nil
	}
	if err := s.values.Erase(q); err != nil {
		return err
	}
	s.trail = append(s.trail, quantifierSetTrail{value: q})
	return nil
}

func (s *scopedQuantifierSet) pushScope() {
	s.scopes = append(s.scopes, len(s.trail))
}

func (s *scopedQuantifierSet) popScope() error {
	if len(s.scopes) == 0 {
		return ast.ErrASTDomain
	}
	last := len(s.scopes) - 1
	stop := s.scopes[last]
	s.scopes = s.scopes[:last]
	for len(s.trail) > stop {
		i := len(s.trail) - 1
		edit := s.trail[i]
		if s.onUndo != nil {
			s.onUndo(edit.value, edit.inserted)
		}
		var err error
		if edit.inserted {
			err = s.values.Erase(edit.value)
		} else {
			err = s.values.Insert(edit.value, [2]ast.ASTHandle{})
		}
		if err != nil {
			return err
		}
		s.trail = s.trail[:i]
	}
	return nil
}

func (s *scopedQuantifierSet) reset() {
	s.values.Reset()
	s.trail = s.trail[:0]
	s.scopes = s.scopes[:0]
}

// snapshot lists the members in iteration order.
func (s *scopedQuantifierSet) snapshot() []ast.ASTHandle {
	entries := s.values.Entries()
	result := make([]ast.ASTHandle, len(entries))
	for i, entry := range entries {
		result[i] = entry.Key
	}
	return result
}
