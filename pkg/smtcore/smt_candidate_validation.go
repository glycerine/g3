package smtcore

// Source: src/smt/smt_context_inv.cpp context::validate_model.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

func (c *SMTModelChecker) validateGroundAssignments(md *protomodel.ProtoModel) (valid bool, result error) {
	for _, literal := range c.state.assignedLiterals {
		expression, err := c.state.BoolExpression(literal.Var())
		if err != nil {
			return false, err
		}
		if !c.state.isRelevant(expression) {
			continue
		}
		valid, done, err := c.validateLiteral(md, literal.Sign(), expression)
		if err != nil || done {
			return valid, err
		}
	}
	return true, nil
}

// validateLiteral is one iteration of the validate_model loop: the locals
// expr_ref n, res end with it (res first). An assigned literal is l_true.
func (c *SMTModelChecker) validateLiteral(md *protomodel.ProtoModel, negative bool, expression ast.ASTHandle) (valid, done bool, result error) {
	m := c.manager
	// literal2expr(lit, n)
	n := expression
	if negative {
		var err error
		if n, err = m.MkNot(expression); err != nil {
			return false, true, err
		}
	}
	nRef, err := m.NewRef(n)
	if err != nil {
		return false, true, err
	}
	res, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, true, errors.Join(err, m.DelRef(&nRef))
	}
	defer func() { result = errors.Join(result, m.DelRef(&res), m.DelRef(&nRef)) }()
	ground, err := m.IsGround(n)
	if err != nil || !ground {
		return false, err != nil, err
	}
	kind, err := m.GetKind(n)
	if err != nil {
		return false, true, err
	}
	if kind == ast.ASTQuantifier {
		recursive, err := m.IsRecursiveFunctionDefinition(n)
		if err != nil {
			return false, true, err
		}
		if recursive {
			return false, false, nil
		}
	}
	success, err := md.Eval(n, res, false)
	if err != nil {
		return false, true, err
	}
	if !success {
		return true, true, nil
	}
	value, err := m.RefGet(res)
	if err != nil {
		return false, true, err
	}
	if m.IsFalse(value) {
		return false, true, nil
	}
	return false, false, nil
}
