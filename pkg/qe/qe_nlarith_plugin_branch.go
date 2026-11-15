package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Assign is nlarith_plugin::assign: the branch formula is factored
// (m_factor_rw) and simplified (m_rewriter) before add_constraint(true, ...).
func (p *NonlinearArithmeticPlugin) Assign(variable *ast.ContainsApp, formula ast.ASTHandle, value *numeral.OwnedRational) (err error) {
	if value == nil {
		return ast.ErrASTHandle
	}
	e, _, err := p.entry(variable, formula)
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
	branchFormula, err := p.manager.RefGet(e.branches.Branches[branch].Branch)
	if err != nil {
		return err
	}
	// expr_ref result(m), tmp(m);
	tmp, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, p.manager.DelRef(&tmp)) }()
	result, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			err = errors.Join(err, p.manager.DelRef(&result))
		}
	}()
	// m_factor_rw(branch_fml, tmp); m_rewriter(tmp, result);
	if err = p.factor.Rewrite(branchFormula, tmp); err != nil {
		return err
	}
	factored, err := p.manager.RefGet(tmp)
	if err != nil {
		return err
	}
	if err = p.rewriter.Rewrite(factored, result); err != nil {
		return err
	}
	h, err := p.manager.RefGet(result)
	if err != nil {
		return err
	}
	if done, err := p.context.addConstraint(true, h); done || err != nil {
		return err
	}
	keep = true
	p.constraints = append(p.constraints, ArithmeticConstraint{UseVariable: true, Formula: result})
	return nil
}
func (p *NonlinearArithmeticPlugin) TakeConstraints() ([]ArithmeticConstraint, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	r := p.constraints
	p.constraints = nil
	return r, nil
}
