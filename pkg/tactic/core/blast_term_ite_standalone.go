package core

// Source: src/tactic/core/blast_term_ite_tactic.cpp static blast_term_ite.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// BlastTermITEInto hoists non-Boolean ITEs into the caller's owning reference.
// The result stays rooted after the temporary traversal is destroyed. Like
// the source standalone helper, rewriting does not produce proofs; the
// manager's proof mode is left unchanged.
// Source: src/tactic/core/blast_term_ite_tactic.cpp:189-197,204-206.
func BlastTermITEInto(m *ast.ASTManager, formula ast.ASTHandle, out ast.ASTRef) (err error) {
	if m == nil || formula.IsNull() {
		return ast.ErrASTHandle
	}
	if _, err = m.RefGet(out); err != nil {
		return err
	}
	owner := &BlastTermITETactic{manager: m}
	config := blastTermITEConfiguration{owner: owner, standalone: true}
	traversal, err := rewriter.NewTraversal(m, &config)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, traversal.Close()) }()
	return traversal.Rewrite(formula, out)
}

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
