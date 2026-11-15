package interp

// Source checkpoint: src/interp/iz3mgr.cpp z3_simplify and z3_really_simplify.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// rewrite is z3_simplify: a local th_rewriter rewrites term and dies, with
// its cache, once the result is held.
func (m *Manager) rewrite(term ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if err := m.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if !m.rewriting {
		return ast.ASTRef{}, NewError("interpolation manager has no theory rewriter")
	}
	rewriter, err := astrewriter.NewTheoryRewriter(m.ast, m.context, m.globals, nil)
	if err != nil {
		// A refused constructor still returns its partial owner.
		if rewriter != nil {
			err = errors.Join(err, rewriter.Close())
		}
		return ast.ASTRef{}, err
	}
	out, err := m.ast.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, rewriter.Close())
	}
	if err = rewriter.Rewrite(term, out); err != nil {
		return ast.ASTRef{}, errors.Join(err, m.ast.DelRef(&out), rewriter.Close())
	}
	if err = rewriter.Close(); err != nil {
		return ast.ASTRef{}, errors.Join(err, m.ast.DelRef(&out))
	}
	return out, nil
}

// Simplify is z3_simplify. No retained site calls z3_really_simplify: iZ3
// calls z3_simplify (AUDIT.md interp_02-F2), and the Duality variable
// projector's simp_params sets only ":som"/":sort-sums", which th_rewriter
// never reads.
func (m *Manager) Simplify(term ast.ASTHandle) (ast.ASTRef, error) {
	return m.rewrite(term)
}
