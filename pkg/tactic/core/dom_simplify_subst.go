package core

// Source: src/tactic/core/dom_simplify_tactic.cpp:444-552 and
// dom_simplify_tactic.h:149-178 (expr_substitution_simplifier).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// exprSubstitutionSimplifier is expr_substitution_simplifier, the
// dom_simplifier that mk_dom_simplify_tactic installs.
type exprSubstitutionSimplifier struct {
	manager *ast.ASTManager
	subst   ast.ASTExprSubstitution       // m_subst
	scoped  ast.ASTScopedExprSubstitution // m_scoped_substitution
	trail   []ast.ASTRef                  // m_trail
	closed  bool
}

func newExprSubstitutionSimplifier(m *ast.ASTManager) (*exprSubstitutionSimplifier, error) {
	subst, err := m.NewExprSubstitution()
	if err != nil {
		return nil, err
	}
	scoped, err := m.NewScopedExprSubstitution(subst)
	if err != nil {
		return nil, errors.Join(err, m.DelExprSubstitution(&subst))
	}
	return &exprSubstitutionSimplifier{manager: m, subst: subst, scoped: scoped}, nil
}

// AssertExpression is expr_substitution_simplifier::assert_expr. It always
// pushes one scope and always reports success.
func (s *exprSubstitutionSimplifier) AssertExpression(t ast.ASTHandle, sign bool) (bool, error) {
	m := s.manager
	if err := m.ScopedExprSubstitutionPush(s.scoped); err != nil {
		return false, err
	}
	if !sign {
		return true, s.updateSubstitution(t)
	}
	yes, tt, err := m.IsNot(t)
	if err != nil {
		return false, err
	}
	if yes {
		return true, s.updateSubstitution(tt)
	}
	nt, err := m.MkNot(t)
	if err != nil {
		return false, err
	}
	ref, err := m.NewRef(nt)
	if err != nil {
		return false, err
	}
	err = s.updateSubstitution(nt)
	return true, errors.Join(err, m.DelRef(&ref))
}

// isGreater is expr_substitution_simplifier::is_gt. The source depth table
// (compute_depth) agrees with the AST depth on the ground applications that
// reach it.
func (s *exprSubstitutionSimplifier) isGreater(lhs, rhs ast.ASTHandle) (bool, error) {
	m := s.manager
	if lhs == rhs {
		return false, nil
	}
	value, err := m.IsValue(rhs)
	if err != nil {
		return false, err
	}
	if value {
		return true, nil
	}
	ld, err := m.GetDepth(lhs)
	if err != nil {
		return false, err
	}
	rd, err := m.GetDepth(rhs)
	if err != nil {
		return false, err
	}
	if ld > rd {
		return true, nil
	}
	lk, err := m.GetKind(lhs)
	if err != nil {
		return false, err
	}
	rk, err := m.GetKind(rhs)
	if err != nil {
		return false, err
	}
	if ld != rd || lk != ast.ASTApp || rk != ast.ASTApp {
		return false, nil
	}
	l, err := m.GetDecl(lhs)
	if err != nil {
		return false, err
	}
	r, err := m.GetDecl(rhs)
	if err != nil {
		return false, err
	}
	lid, err := m.GetASTID(l)
	if err != nil {
		return false, err
	}
	rid, err := m.GetASTID(r)
	if err != nil {
		return false, err
	}
	if lid != rid {
		return lid > rid, nil
	}
	ln, err := m.GetNumArgs(lhs)
	if err != nil {
		return false, err
	}
	rn, err := m.GetNumArgs(rhs)
	if err != nil {
		return false, err
	}
	if ln != rn {
		return ln > rn, nil
	}
	for i := uint32(0); i < ln; i++ {
		la, err := m.GetArg(lhs, i)
		if err != nil {
			return false, err
		}
		ra, err := m.GetArg(rhs, i)
		if err != nil {
			return false, err
		}
		if la != ra {
			return s.isGreater(la, ra)
		}
	}
	return false, ast.ErrASTDomain // UNREACHABLE
}

func (s *exprSubstitutionSimplifier) insert(key, def ast.ASTHandle) error {
	return s.manager.ScopedExprSubstitutionInsert(s.scoped, key, def, ast.ASTHandle{}, ast.ASTDependency{})
}

// updateSubstitution is expr_substitution_simplifier::update_substitution
// with the null proof that assert_expr passes.
func (s *exprSubstitutionSimplifier) updateSubstitution(n ast.ASTHandle) error {
	m := s.manager
	ground, err := m.IsGround(n)
	if err != nil {
		return err
	}
	if ground {
		equality, err := domOp(m, n, ast.ASTOpEq)
		if err != nil {
			return err
		}
		if !equality {
			if equality, err = domOp(m, n, ast.ASTOpIff); err != nil {
				return err
			}
		}
		if equality {
			count, err := m.GetNumArgs(n)
			if err != nil {
				return err
			}
			equality = count == 2
		}
		if equality {
			lhs, err := m.GetArg(n, 0)
			if err != nil {
				return err
			}
			rhs, err := m.GetArg(n, 1)
			if err != nil {
				return err
			}
			for _, h := range [2]ast.ASTHandle{lhs, rhs} {
				ref, err := m.NewRef(h)
				if err != nil {
					return err
				}
				s.trail = append(s.trail, ref)
			}
			greater, err := s.isGreater(lhs, rhs)
			if err != nil {
				return err
			}
			if greater {
				return s.insert(lhs, rhs)
			}
			if greater, err = s.isGreater(rhs, lhs); err != nil {
				return err
			}
			if greater {
				return s.insert(rhs, lhs)
			}
		}
	}
	yes, n1, err := m.IsNot(n)
	if err != nil {
		return err
	}
	if yes {
		falsity, err := m.MkFalse()
		if err != nil {
			return err
		}
		return s.insert(n1, falsity)
	}
	truth, err := m.MkTrue()
	if err != nil {
		return err
	}
	return s.insert(n, truth)
}

// Rewrite is expr_substitution_simplifier::operator(): one lookup.
func (s *exprSubstitutionSimplifier) Rewrite(out ast.ASTRef) error {
	h, err := s.manager.RefGet(out)
	if err != nil {
		return err
	}
	v, err := s.manager.ScopedExprSubstitutionFindValue(s.scoped, h)
	if err != nil || v == h {
		return err
	}
	return s.manager.SetRef(out, v)
}

func (s *exprSubstitutionSimplifier) Pop(n uint32) error {
	return s.manager.ScopedExprSubstitutionPop(s.scoped, n)
}

func (s *exprSubstitutionSimplifier) ScopeLevel() uint32 {
	n, err := s.manager.ScopedExprSubstitutionScopeLevel(s.scoped)
	if err != nil {
		return 0
	}
	return n
}

func (s *exprSubstitutionSimplifier) TranslateDom(m *ast.ASTManager) (DomSimplifier, error) {
	return newExprSubstitutionSimplifier(m)
}

func (s *exprSubstitutionSimplifier) UpdateParams(*uparams.ParamsRef) error { return nil }

func (s *exprSubstitutionSimplifier) Close() error {
	if s.closed {
		return nil
	}
	m := s.manager
	err := m.ScopedExprSubstitutionPop(s.scoped, s.ScopeLevel())
	for i := range s.trail {
		err = errors.Join(err, m.DelRef(&s.trail[i]))
	}
	s.trail = nil
	err = errors.Join(err, m.DelScopedExprSubstitution(&s.scoped), m.DelExprSubstitution(&s.subst))
	s.closed = true
	return err
}
