package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
)

// EliminateExistentialDefinitions returns an owning reference to an equivalent
// formula with equality-defined existential variables eliminated where possible.
func (c *SMTContext) EliminateExistentialDefinitions(h ast.ASTHandle) (ast.ASTRef, error) {
	return c.rewriteVerificationDefinitions(h, qe.EliminateExistentialDefinitions)
}

// SimplifyVerificationCondition returns an owning reference to an equivalent
// formula after eliminating equality-defined witnesses and guarded universals.
func (c *SMTContext) SimplifyVerificationCondition(h ast.ASTHandle) (ast.ASTRef, error) {
	return c.rewriteVerificationDefinitions(h, qe.SimplifyVerificationCondition)
}

func (c *SMTContext) rewriteVerificationDefinitions(h ast.ASTHandle, rewrite func(*ast.ASTManager, qe.Config, ast.ASTRef) error) (ast.ASTRef, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	r, err := m.NewRef(h)
	if err != nil {
		return ast.ASTRef{}, err
	}
	cfg, err := c.apiQEConfig()
	if err == nil {
		err = rewrite(m, cfg, r)
	}
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&r))
	}
	return r, nil
}
