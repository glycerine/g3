package arith

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"testing"
)

func statisticGoal(t *testing.T, m *ast.ASTManager, forms ...ast.ASTHandle) *tactic.Goal {
	t.Helper()
	return fixDLGoal(t, m, false, false, false, forms...)
}
func statisticEquality(t *testing.T, m *ast.ASTManager, a, b ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	h, e := m.MkEq(a, b)
	equationMust(t, e)
	fixDLPin(t, m, h)
	return h
}
