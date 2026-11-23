package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func goalTestNew(t *testing.T, m *ast.ASTManager, proofs, cores bool) *tactic.Goal {
	t.Helper()
	g, e := tactic.NewGoal(m, proofs, true, cores)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, g.Close()) })
	return g
}
func goalTestAtom(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	h := proofTestBool(t, m, name)
	astTestPin(t, m, h)
	return h
}

func goalTestApp(t *testing.T, m *ast.ASTManager, op int32, args ...ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	h, e := m.MkFamilyApp(0, op, nil, args)
	astTestMust(t, e)
	astTestPin(t, m, h)
	return h
}

func proofTestBool(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	s, e := m.MkBoolSort()
	astTestMust(t, e)
	h, e := m.MkConst(astTestSymbol(t, m, name), s)
	astTestMust(t, e)
	return h
}
func astTestPin(t testing.TB, m *ast.ASTManager, h ast.ASTHandle) ast.ASTRef {
	t.Helper()
	r, e := m.NewRef(h)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&r)) })
	return r
}
