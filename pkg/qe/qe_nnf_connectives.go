package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// gateRef is bool_rewriter mk_and/mk_or into a caller-owned expr_ref.
func (n *QENNFNormalizer) gateRef(operation int32, arguments ...ast.ASTHandle) (ast.ASTRef, error) {
	out, err := n.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return out, err
	}
	if err = astrewriter.SimplifyBooleanGateDefault(n.manager, operation, arguments, out); err != nil {
		_ = n.manager.DelRef(&out)
		return ast.ASTRef{}, err
	}
	return out, nil
}
