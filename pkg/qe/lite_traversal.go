package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// This is the retained concrete elim_cfg specialization. All ordinary nodes
// use the existing source defaults and the original typed traversal engine.
type liteConfiguration struct {
	rewriter.DefaultConfiguration
	owner *Lite
}

func (cfg *liteConfiguration) ReduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (bool, error) {
	l := cfg.owner
	c := l.active
	if c == nil {
		return false, ast.ErrASTDomain
	}
	m := l.manager
	forall, err := m.IsForall(q)
	if err != nil {
		return false, err
	}
	// Every step assigns the single expr_ref result, releasing what it held.
	if err = m.SetRef(out, body); err != nil {
		return false, err
	}
	current := body
	if forall {
		if current, err = m.MkNot(current); err != nil {
			return false, err
		}
		if err = m.SetRef(out, current); err != nil {
			return false, err
		}
	}
	n, err := m.GetNumDeclarations(q)
	if err != nil {
		return false, err
	}
	indices := make([]uint32, n)
	for i := range indices {
		indices[i] = uint32(i)
	}
	if err = l.eliminateExpression(c, indices, true, out); err != nil {
		return false, err
	}
	if current, err = m.RefGet(out); err != nil {
		return false, err
	}
	if forall {
		if current, err = m.PushNot(current); err != nil {
			return false, err
		}
		if err = m.SetRef(out, current); err != nil {
			return false, err
		}
	}
	pats := make([]ast.ASTHandle, patterns.Count())
	nps := make([]ast.ASTHandle, noPatterns.Count())
	for i := range pats {
		pats[i], err = patterns.At(uint32(i))
		if err != nil {
			return false, err
		}
	}
	for i := range nps {
		nps[i], err = noPatterns.At(uint32(i))
		if err != nil {
			return false, err
		}
	}
	updated, err := m.UpdateQuantifierAnnotations(q, pats, nps, current)
	if err != nil {
		return false, err
	}
	if err = m.SetRef(out, updated); err != nil {
		return false, err
	}
	// m_imp.m_rewriter(result), in place.
	return true, l.rewriter.Rewrite(updated, out)
}

// Rewrite performs the complete source elim_star round. As in the source's
// no-proof specialization, the supplied proof reference is left unchanged.
func (l *Lite) Rewrite(formula, proof ast.ASTRef) (err error) {
	c, err := l.begin()
	if err != nil {
		return err
	}
	defer l.finish(c, &err)
	l.active = c
	defer func() { l.active = nil }()
	h, err := l.manager.RefGet(formula)
	if err != nil {
		return err
	}
	tmp, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return err
	}
	if err = l.traversal.Rewrite(h, tmp); err != nil {
		return err
	}
	h, err = l.manager.RefGet(tmp)
	if err != nil {
		return err
	}
	return l.manager.SetRef(formula, h)
}
