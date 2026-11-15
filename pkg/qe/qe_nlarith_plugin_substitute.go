package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Substitute is nlarith_plugin::subst: every predicate is replaced by its
// branch substitution in one simultaneous expr_safe_replace pass, the
// result is conjoined with the branch constraint and factored
// (m_factor_rw), and the definition is factored as well.
func (p *NonlinearArithmeticPlugin) Substitute(variable *ast.ContainsApp, value *numeral.OwnedRational, formula ast.ASTRef, definition *ast.ASTRef) error {
	if value == nil {
		return ast.ErrASTHandle
	}
	root, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	e, _, err := p.entry(variable, root)
	if err != nil {
		return err
	}
	branch, err := p.manager.NumeralService().Unsigned(value)
	if err != nil || int(branch) >= len(e.branches.Branches) {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	row := e.branches.Branches[branch]
	if len(row.Substitutions) != len(e.branches.Predicates) {
		return ast.ErrASTDomain
	}
	// m_replace.reset(); m_replace.insert(preds(i), subst(j)[i]) for each i;
	// m_replace(fml).
	if err = p.replacer.Reset(); err != nil {
		return err
	}
	for i := range row.Substitutions {
		predicate, cause := p.manager.RefGet(e.branches.Predicates[i])
		if cause != nil {
			return cause
		}
		replacement, cause := p.manager.RefGet(row.Substitutions[i])
		if cause != nil {
			return cause
		}
		if cause = p.replacer.Insert(predicate, replacement); cause != nil {
			return cause
		}
	}
	if err = p.replacer.ApplyRef(formula); err != nil {
		return err
	}
	rewritten, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	constraint, err := p.manager.RefGet(row.Constraint)
	if err != nil {
		return err
	}
	// expr_ref tmp(m.mk_and(brs->constraints(j), fml), m); m_factor_rw(tmp, fml);
	combined, err := p.manager.MkAnd(constraint, rewritten)
	if err != nil {
		return err
	}
	if err = p.manager.SetRef(formula, combined); err != nil {
		return err
	}
	if err = p.factor.Rewrite(combined, formula); err != nil {
		return err
	}
	if definition != nil {
		// m_factor_rw(brs->def(j), *def);
		d, err := p.manager.RefGet(row.Definition)
		if err != nil {
			return err
		}
		return p.factor.Rewrite(d, *definition)
	}
	return nil
}
