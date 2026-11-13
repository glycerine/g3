package tab

// Source: src/muz/tab/tab_context.cpp tb::unifier's members (m_S1, m_S2,
// m_rename, m_sub1, m_sub2). tab::imp keeps one unifier for the engine's
// life; each unify starts with reset(), so the previous unification's
// substitution results, renaming, and proof substitutions stay alive until
// the next one.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/ast/substitution"
)

type unifierMembers struct {
	manager *ast.ASTManager
	s1      *substitution.Substitution
	s2      *rewriter.VarSubstitution
	rename  []ast.ASTRef
	sub     *proofSubstitutions
}

func (u *unifierMembers) substitution() (*substitution.Substitution, error) {
	if u.s1 == nil {
		s, err := substitution.New(u.manager)
		if err != nil {
			return nil, err
		}
		u.s1 = s
	}
	return u.s1, nil
}

func (u *unifierMembers) varSubstitution() (*rewriter.VarSubstitution, error) {
	if u.s2 == nil {
		s, err := rewriter.NewVarSubstitution(u.manager, false)
		if err != nil {
			return nil, err
		}
		u.s2 = s
	}
	return u.s2, nil
}

// reset is unifier::reset: m_S1, m_S2, m_rename, m_sub1, m_sub2 in order.
func (u *unifierMembers) reset() error {
	if u == nil {
		return nil
	}
	if u.s1 != nil {
		if err := u.s1.Reset(); err != nil {
			return err
		}
	}
	if u.s2 != nil {
		if err := u.s2.Reset(); err != nil {
			return err
		}
	}
	for len(u.rename) != 0 {
		if u.rename[0] != (ast.ASTRef{}) {
			if err := u.manager.DelRef(&u.rename[0]); err != nil {
				return err
			}
		}
		u.rename = u.rename[1:]
	}
	u.rename = nil
	if u.sub != nil {
		if err := u.sub.Close(); err != nil {
			return err
		}
		u.sub = nil
	}
	return nil
}

func (u *unifierMembers) Close() error {
	if u == nil {
		return nil
	}
	if err := u.reset(); err != nil {
		return err
	}
	if u.s1 != nil {
		if err := u.s1.Close(); err != nil {
			return err
		}
		u.s1 = nil
	}
	if u.s2 != nil {
		if err := u.s2.Close(); err != nil {
			return err
		}
		u.s2 = nil
	}
	return nil
}
