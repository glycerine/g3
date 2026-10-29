package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func pbAPIFixture(t *testing.T) (*smtAPIFixture, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	f := newSMTAPIFixture(t, "QF_FD", nil)
	boolean := f.pin(f.m.MkBoolSort())
	constant := func(name string) ast.ASTHandle {
		symbol, err := f.m.Symbols.Intern(name)
		astTestMust(t, err)
		return f.pin(f.m.MkConst(symbol, boolean))
	}
	return f, constant("api-pb-a"), constant("api-pb-b")
}

func pbAPIKind(t *testing.T, m *ast.ASTManager, formula ast.ASTHandle) int32 {
	t.Helper()
	kind, err := m.GetDeclKind(formula)
	astTestMust(t, err)
	return kind
}
