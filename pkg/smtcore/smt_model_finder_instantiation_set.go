package smtcore

// Source: src/smt/smt_model_finder.cpp instantiation_set.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// smtModelFinderInstantiationSet is instantiation_set: m_elems holds a
// reference to each term (Key) with its generation (Num), and m_inv maps a
// value (Key) to its term (Value[0]). Both are obj_maps, iterated in table
// order. m_inv holds no references: its values are owned by the
// evaluator's cache.
type smtModelFinderInstantiationSet struct {
	manager  *ast.ASTManager
	elements *ast.ObjMap
	inverse  *ast.ObjMap
	closed   bool
}

func newSMTModelFinderInstantiationSet(manager *ast.ASTManager) (*smtModelFinderInstantiationSet, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &smtModelFinderInstantiationSet{
		manager: manager, elements: ast.NewObjMap(manager), inverse: ast.NewObjMap(manager),
	}, nil
}

func (s *smtModelFinderInstantiationSet) open() error {
	if s == nil || s.manager == nil || s.elements == nil || s.closed {
		return ast.ErrASTHandle
	}
	return nil
}

// entries is get_elems() in iteration order.
func (s *smtModelFinderInstantiationSet) entries() []ast.ObjMapEntry {
	return s.elements.Entries()
}

func (s *smtModelFinderInstantiationSet) size() int { return s.elements.Len() }

// generation is get_generation(t).
func (s *smtModelFinderInstantiationSet) generation(term ast.ASTHandle) (uint32, error) {
	gen, _, err := s.elements.FindNum(term)
	return uint32(gen), err
}

func (s *smtModelFinderInstantiationSet) containsModelValue(root ast.ASTHandle) (bool, error) {
	return containsPrivateModelTerm(s.manager, root, false)
}

func containsPrivateModelTerm(manager *ast.ASTManager, root ast.ASTHandle, rejectFunctionArrays bool) (bool, error) {
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]struct{})
	for len(stack) != 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, found := seen[n]; found {
			continue
		}
		seen[n] = struct{}{}
		value, err := manager.IsModelValue(n)
		if err != nil {
			return false, err
		}
		if value {
			return true, nil
		}
		kind, err := manager.GetKind(n)
		if err != nil {
			return false, err
		}
		if kind == ast.ASTQuantifier {
			body, err := manager.GetQuantifierBody(n)
			if err != nil {
				return false, err
			}
			stack = append(stack, body)
			patterns, err := manager.GetNumPatterns(n)
			if err != nil {
				return false, err
			}
			for i := uint32(0); i < patterns; i++ {
				pattern, err := manager.GetPattern(n, i)
				if err != nil {
					return false, err
				}
				stack = append(stack, pattern)
			}
			noPatterns, err := manager.GetNumNoPatterns(n)
			if err != nil {
				return false, err
			}
			for i := uint32(0); i < noPatterns; i++ {
				pattern, err := manager.GetNoPattern(n, i)
				if err != nil {
					return false, err
				}
				stack = append(stack, pattern)
			}
			continue
		}
		if kind != ast.ASTApp {
			continue
		}
		if rejectFunctionArrays {
			family, err := manager.GetFamilyID(n)
			if err != nil {
				return false, err
			}
			arrayFamily, err := manager.ArrayFamilyID()
			if err != nil {
				return false, err
			}
			if family == arrayFamily {
				op, err := manager.GetDeclKind(n)
				if err != nil {
					return false, err
				}
				if op == ast.ASTArrayOpAsArray {
					return true, nil
				}
			}
		}
		count, err := manager.GetNumArgs(n)
		if err != nil {
			return false, err
		}
		for i := uint32(0); i < count; i++ {
			child, err := manager.GetArg(n, i)
			if err != nil {
				return false, err
			}
			stack = append(stack, child)
		}
	}
	return false, nil
}

func (s *smtModelFinderInstantiationSet) Insert(term ast.ASTHandle, generation uint32) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if found, err := s.elements.Contains(term); err != nil || found {
		return false, err
	}
	contains, err := s.containsModelValue(term)
	if err != nil || contains {
		return false, err
	}
	if err = s.manager.IncRef(term); err != nil {
		return false, err
	}
	if err = s.elements.InsertNum(term, uint64(generation)); err != nil {
		return false, errors.Join(err, s.manager.DecRef(term))
	}
	return true, nil
}

func (s *smtModelFinderInstantiationSet) Remove(term ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.inverse.Len() != 0 {
		return ast.ErrASTDomain
	}
	if found, err := s.elements.Contains(term); err != nil || !found {
		return errors.Join(err, ast.ErrASTDomain)
	}
	if err := s.elements.Erase(term); err != nil {
		return err
	}
	return s.manager.DecRef(term)
}

// MakeInverse is mk_inverse(ev): eval returns a null handle when the
// evaluation fails, which stops the walk.
func (s *smtModelFinderInstantiationSet) MakeInverse(eval func(ast.ASTHandle) (ast.ASTHandle, error)) error {
	if err := s.open(); err != nil {
		return err
	}
	if eval == nil {
		return ast.ErrASTHandle
	}
	for _, entry := range s.elements.Entries() {
		term, generation := entry.Key, uint32(entry.Num)
		value, err := eval(term)
		if err != nil {
			return err
		}
		if value.IsNull() {
			break
		}
		old, found, err := s.inverse.Find(value)
		if err != nil {
			return err
		}
		if found {
			oldGeneration, err := s.generation(old[0])
			if err != nil {
				return err
			}
			if generation < oldGeneration {
				if err = s.inverse.Insert(value, [2]ast.ASTHandle{term}); err != nil {
					return err
				}
			}
			continue
		}
		if err = s.inverse.Insert(value, [2]ast.ASTHandle{term}); err != nil {
			return err
		}
	}
	return nil
}

func (s *smtModelFinderInstantiationSet) Inverse(value ast.ASTHandle) (term ast.ASTHandle, generation uint32, found bool, err error) {
	if err = s.open(); err != nil {
		return term, 0, false, err
	}
	inverse, found, err := s.inverse.Find(value)
	if err != nil || !found {
		return term, 0, false, err
	}
	generation, err = s.generation(inverse[0])
	return inverse[0], generation, true, err
}

// Values lists the keys of get_inv_map() in iteration order.
func (s *smtModelFinderInstantiationSet) Values() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, 0, s.inverse.Len())
	for _, entry := range s.inverse.Entries() {
		result = append(result, entry.Key)
	}
	return result, nil
}

// Close is ~instantiation_set: each element in m_elems order.
func (s *smtModelFinderInstantiationSet) Close() (result error) {
	if s == nil {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if s.manager == nil {
		return ast.ErrASTHandle
	}
	for _, entry := range s.elements.Entries() {
		result = errors.Join(result, s.manager.DecRef(entry.Key))
	}
	if result == nil {
		s.closed = true
		s.elements = nil
		s.inverse = nil
		s.manager = nil
	}
	return result
}
