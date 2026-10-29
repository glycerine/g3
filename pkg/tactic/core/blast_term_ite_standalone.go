package core

// Source: src/tactic/core/blast_term_ite_tactic.cpp static blast_term_ite.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// BlastTermITE hoists non-Boolean ITEs from one expression without changing
// manager proof mode. It returns a borrowed AST handle owned by the manager.
func BlastTermITE(m *ast.ASTManager, formula ast.ASTHandle) (_ ast.ASTHandle, err error) {
	if m == nil || formula.IsNull() {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	owner := &BlastTermITETactic{manager: m}
	config := blastTermITEConfiguration{owner: owner, standalone: true}
	traversal, err := rewriter.NewTraversal(m, &config)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, traversal.Close()) }()
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&out)) }()
	if err = traversal.Rewrite(formula, out); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.RefGet(out)
}
