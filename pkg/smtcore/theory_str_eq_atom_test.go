package smtcore

import "github.com/glycerine/g3/pkg/ast"

// stringEqAtom is context::mk_eq_atom as theory_str uses it: arithmetic sides
// go through arith_util::mk_eq; any other sort puts the smaller id left.
func stringEqAtom(m *ast.ASTManager, lhs, rhs ast.ASTHandle) (ast.ASTHandle, error) {
	sort, err := m.GetSort(lhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := m.GetFamilyID(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arith, err := m.ArithmeticFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if family == arith {
		return smtArithUtilMkEq(m, lhs, rhs)
	}
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
