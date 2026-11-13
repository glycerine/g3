package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *ArithmeticPlugin) Substitute(variable *ast.ContainsApp, value *numeral.OwnedRational, formula ast.ASTRef, definition *ast.ASTRef) error {
	if value == nil {
		return ast.ErrASTHandle
	}
	if err := p.open(); err != nil {
		return err
	}
	root, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	if variable == nil {
		return ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return err
	}
	branchNumber, err := p.manager.NumeralService().Unsigned(value)
	if err != nil {
		return err
	}
	v := new(big.Rat).SetInt64(int64(branchNumber))
	b, err := p.branch(target, root, v)
	if err != nil {
		return err
	}
	// subst: get_def runs before get_cache replaces fml.
	if definition != nil {
		s, err := b.substitution(p.manager)
		if err != nil {
			return err
		}
		d, err := p.branchDefinition(b.Selection, s)
		if err != nil {
			return err
		}
		h := ast.ASTHandle{}
		if d != (ast.ASTRef{}) {
			if h, err = p.manager.RefGet(d); err == nil {
				err = p.manager.SetRef(*definition, h)
			}
			if err = errors.Join(err, p.manager.DelRef(&d)); err != nil {
				return err
			}
		} else if err = p.manager.SetRef(*definition, h); err != nil {
			return err
		}
	}
	result, err := p.manager.RefGet(b.Result)
	if err != nil {
		return err
	}
	if err = p.manager.SetRef(formula, result); err != nil {
		return err
	}
	// get_cache: after result = bf.m_result, add_var(v, false) republishes
	// each variable the branch introduced, so subst adds them again.
	return p.replayBranch(b)
}

func (p *ArithmeticPlugin) Project(variable *ast.ContainsApp, value *numeral.OwnedRational, formula ast.ASTRef) (bool, error) {
	if err := p.Substitute(variable, value, formula, nil); err != nil {
		return false, err
	}
	return true, nil
}
