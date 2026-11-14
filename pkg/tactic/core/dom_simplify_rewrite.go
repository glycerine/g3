package core

import "github.com/glycerine/g3/pkg/ast"

func domOp(m *ast.ASTManager, h ast.ASTHandle, op int32) (bool, error) {
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return false, e
	}
	f, e := m.GetFamilyID(h)
	if e != nil || f != m.BasicFamilyID() {
		return false, e
	}
	v, e := m.GetDeclKind(h)
	return v == op, e
}
