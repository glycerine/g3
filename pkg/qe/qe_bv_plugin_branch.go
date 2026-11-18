package qe

// Source: src/qe/qe_bv_plugin.cpp assign/get_num_branches.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *BitVectorPlugin) GetNumBranches(variable *ast.ContainsApp, formula ast.ASTHandle, branches *numeral.OwnedRational) (bool, error) {
	if branches == nil {
		return false, ast.ErrASTHandle
	}
	if err := p.checkInputs(variable, formula); err != nil {
		return false, err
	}
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	width, err := p.manager.BitVectorSize(target)
	if err != nil {
		return false, err
	}
	// num_branches = rational::power_of_two(sz)
	q := p.manager.NumeralService().Manager
	two := numeral.NewRational()
	err = q.SetInt32(&two, 2)
	if err == nil {
		err = q.Power(&two, width, &branches.Raw)
	}
	if err = errors.Join(err, q.Del(&two)); err != nil {
		return false, err
	}
	return true, nil
}

// Assign is intentionally empty in the source BV plugin.
func (p *BitVectorPlugin) Assign(variable *ast.ContainsApp, formula ast.ASTHandle, _ *numeral.OwnedRational) error {
	return p.checkInputs(variable, formula)
}
