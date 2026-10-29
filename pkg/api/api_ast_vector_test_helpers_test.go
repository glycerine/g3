package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func astVectorFixture(t *testing.T) (*smtAPIFixture, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	f := newSMTAPIFixture(t, "QF_UF", nil)
	boolean := f.pin(f.m.MkBoolSort())
	constant := func(text string) ast.ASTHandle {
		name, err := f.m.Symbols.Intern(text)
		astTestMust(t, err)
		return f.pin(f.m.MkConst(name, boolean))
	}
	return f, constant("api-vector-a"), constant("api-vector-b")
}
