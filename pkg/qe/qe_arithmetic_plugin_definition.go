package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// boundDefinition solves a*x+t=0 for x. It is the finite-branch definition
// retained by subst and projection.
func (p *ArithmeticPlugin) boundDefinition(bound *ArithmeticBound) (ast.ASTRef, error) {
	if bound == nil {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	q := p.util.q()
	if zero, err := q.IsZero(&bound.Coefficient); err != nil || zero {
		return ast.ASTRef{}, errors.Join(err, ast.ErrASTDomain)
	}
	t, err := p.manager.RefGet(bound.Term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	negative, err := p.manager.MkArithmeticNeg(t)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if one, err := q.IsOne(&bound.Coefficient); err != nil {
		return ast.ASTRef{}, err
	} else if one {
		return p.manager.NewRef(negative)
	}
	denominator, err := p.util.numeralFor(&bound.Coefficient, t)
	if err != nil {
		return ast.ASTRef{}, err
	}
	division, err := p.manager.MkArithmeticDiv(negative, denominator)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.manager.NewRef(division)
}
