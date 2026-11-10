package interp

// Source checkpoint: src/interp/iz3mgr.cpp z3_simplify and z3_really_simplify.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// rewrite is z3_simplify / z3_really_simplify: a local th_rewriter (with
// :som and :sort-sums for the strong form) rewrites term and dies, with its
// cache, once the result is held.
func (m *Manager) rewrite(term ast.ASTHandle, strong bool) (result ast.ASTRef, resultErr error) {
	if err := m.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if !m.rewriting {
		return ast.ASTRef{}, NewError("interpolation manager has no theory rewriter")
	}
	var parameters *uparams.ParamsRef
	if strong {
		value, err := m.context.NewValue()
		if err != nil {
			return ast.ASTRef{}, err
		}
		defer func() { resultErr = errors.Join(resultErr, m.context.Del(&value)) }()
		som, sortSums := "som", "sort_sums"
		if err = m.context.SetBoolName(&value, &som, true); err != nil {
			return ast.ASTRef{}, err
		}
		if err = m.context.SetBoolName(&value, &sortSums, true); err != nil {
			return ast.ASTRef{}, err
		}
		parameters = &value
	}
	rewriter, err := astrewriter.NewTheoryRewriter(m.ast, m.context, m.globals, parameters)
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

func (m *Manager) Simplify(term ast.ASTHandle) (ast.ASTRef, error) {
	return m.rewrite(term, false)
}

func (m *Manager) ReallySimplify(term ast.ASTHandle) (ast.ASTRef, error) {
	return m.rewrite(term, true)
}
