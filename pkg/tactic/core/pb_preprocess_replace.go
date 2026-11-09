package core

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

// replace is pb_preprocess_tactic::replace: skipped when a position already
// holds true (is_valid); otherwise m_r rewrites each position under e -> v.
func (t *PBPreprocessTactic) replace(g *tactic.Goal, r *rewriter.TheoryRewriter, positions []uint32, variable, value ast.ASTHandle) (_ bool, result error) {
	m := t.manager
	for _, index := range positions {
		h, x := g.Form(index)
		if x != nil {
			return false, x
		}
		if m.IsTrue(h) {
			return false, nil
		}
	}
	sub, e := m.NewExprSubstitution()
	if e != nil {
		return false, e
	}
	defer func() { result = errors.Join(result, m.DelExprSubstitution(&sub)) }()
	if e = m.ExprSubstitutionInsert(sub, variable, value, ast.ASTHandle{}, ast.ASTDependency{}); e != nil {
		return false, e
	}
	if e = r.SetSubstitution(&sub); e != nil {
		return false, e
	}
	defer func() { result = errors.Join(result, r.SetSubstitution(nil)) }()
	out, e := m.NewRef(ast.ASTHandle{})
	if e != nil {
		return false, e
	}
	defer func() { result = errors.Join(result, m.DelRef(&out)) }()
	progress := false
	for _, index := range positions {
		h, x := g.Form(index)
		if x != nil {
			return false, x
		}
		if m.IsTrue(h) {
			continue
		}
		if x = r.Rewrite(h, out); x != nil {
			return false, x
		}
		next, x := m.RefGet(out)
		if x != nil {
			return false, x
		}
		if next == h {
			continue
		}
		pr, x := g.Proof(index)
		if x != nil {
			return false, x
		}
		dep, x := g.Dependency(index)
		if x != nil {
			return false, x
		}
		if x = g.Update(index, next, pr, dep); x != nil {
			return false, x
		}
		progress = true
	}
	return progress, nil
}
