package smtcore

// Source: src/smt/smt_context.cpp context::mk_eq_atom, src/smt/smt_theory.h
// theory::mk_eq_atom, and the arithmetic overrides (theory_arith,
// theory_diff_logic, theory_dense_diff_logic, theory_utvpi), which call
// arith_util::mk_eq.

import "github.com/glycerine/g3/pkg/ast"

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
