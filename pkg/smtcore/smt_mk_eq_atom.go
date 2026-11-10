package smtcore

// Source: src/smt/smt_context.cpp context::mk_eq_atom, src/smt/smt_theory.h
// theory::mk_eq_atom, and the arithmetic overrides (theory_arith,
// theory_diff_logic, theory_dense_diff_logic, theory_utvpi), which call
// arith_util::mk_eq.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// MkEqAtom is context::mk_eq_atom(lhs, rhs).
func (s *SMTInternalizerState) MkEqAtom(lhs, rhs ast.ASTHandle) (ast.ASTHandle, error) {
	m := s.manager
	sort, err := m.GetSort(lhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := m.GetFamilyID(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch s.theories[family].(type) {
	case *SMTTheoryArith, *SMTRealDiffTheory, *SMTIntegerDiffTheory, *SMTDenseDiffTheory, *SMTUTVPITheory:
		return smtArithUtilMkEq(m, lhs, rhs)
	}
	// theory::mk_eq_atom, and the context's own fallback, order by id.
	lid, err := m.GetASTID(lhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rid, err := m.GetASTID(rhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if lid > rid {
		lhs, rhs = rhs, lhs
	}
	return m.MkEq(lhs, rhs)
}

// smtArithUtilMkEq is arith_util::mk_eq: numerals go right, otherwise the
// smaller id goes left; identical sides give true, two numerals false.
func smtArithUtilMkEq(m *ast.ASTManager, lhs, rhs ast.ASTHandle) (ast.ASTHandle, error) {
	numeral := func(h ast.ASTHandle) (bool, error) {
		_, _, yes, err := m.ArithmeticNumeralText(h)
		return yes, err
	}
	ln, err := numeral(lhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rn, err := numeral(rhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	lid, err := m.GetASTID(lhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rid, err := m.GetASTID(rhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if ln || (!rn && lid > rid) {
		lhs, rhs = rhs, lhs
		ln, rn = rn, ln
	}
	if lhs == rhs {
		return m.MkTrue()
	}
	if ln && rn {
		return m.MkFalse()
	}
	return m.MkEq(lhs, rhs)
}

// getAssignments is context::get_assignments into a local expr_ref_vector:
// literal2expr of each assigned literal, (not e) for a negative one. The
// returned release frees the vector front to back, as its destructor does.
func (s *SMTInternalizerState) getAssignments() (assignments []ast.ASTHandle, release func() error, err error) {
	m := s.manager
	refs := make([]ast.ASTRef, 0, len(s.assignedLiterals))
	release = func() (result error) {
		for i := range refs {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
		refs = nil
		return result
	}
	for _, lit := range s.assignedLiterals {
		var e ast.ASTHandle
		switch {
		case lit == SMTTrueLiteral:
			e, err = m.MkTrue()
		case lit == SMTFalseLiteral:
			e, err = m.MkFalse()
		default:
			if e, err = s.BoolExpression(lit.Var()); err == nil && lit.Sign() {
				e, err = m.MkNot(e)
			}
		}
		if err != nil {
			return nil, nil, errors.Join(err, release())
		}
		r, e2 := m.NewRef(e)
		if e2 != nil {
			return nil, nil, errors.Join(e2, release())
		}
		refs = append(refs, r)
		assignments = append(assignments, e)
	}
	return assignments, release, nil
}
