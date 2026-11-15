package arith

// Source: src/tactic/goal.h:161-166 (for_each_expr_at) and
// src/ast/for_each_expr.h:27-123, for_each_expr_core<ForEachProc, expr_mark,
// true, false>: every subterm is visited once, including quantifier bodies,
// patterns and no-patterns.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func statisticApplications(g *tactic.Goal) ([]ast.ASTHandle, error) {
	var out []ast.ASTHandle
	_, e := visitArithmeticGoal(g, func(h ast.ASTHandle, kind ast.ASTKind) (bool, error) {
		if kind == ast.ASTApp {
			out = append(out, h)
		}
		return false, nil
	})
	if e != nil {
		return nil, e
	}
	return out, nil
}
