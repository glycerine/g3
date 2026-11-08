package tactic

// Ownership check for values stored in goal arrays (goal_parray.go).

import (
	"github.com/glycerine/g3/pkg/ast"
)

func goalIncExpr(m *ast.ASTManager, h ast.ASTHandle) error {
	// Source expression-reference arrays can contain empty proof entries.
	// Goal formula operations validate nonnull expressions before storage.
	if h.IsNull() {
		return m.IncRef(h)
	}
	kind, e := m.GetKind(h)
	if e != nil {
		return e
	}
	if kind > ast.ASTQuantifier {
		return ast.ErrASTDomain
	}
	return m.IncRef(h)
}
