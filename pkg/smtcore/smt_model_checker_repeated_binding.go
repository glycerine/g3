package smtcore

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

// A restricted auxiliary model can choose different values for two binders
// after the finite checker declined a repeated first binding. Try the second
// value in both positions only if both inverse maps select the same ground term
// and the original model falsifies the quantified body on that tuple.
func (c *SMTModelChecker) tryRepeatedAuxiliaryBinding(md *protomodel.ProtoModel, quantifier ast.ASTHandle, raw, bindings []ast.ASTHandle, generation uint32) (uint32, error) {
	if len(bindings) != 2 || len(raw) != 2 || raw[0] == raw[1] {
		return generation, nil
	}
	m := c.manager
	first, err := m.GetDeclarationSort(quantifier, 0)
	if err != nil {
		return generation, err
	}
	second, err := m.GetDeclarationSort(quantifier, 1)
	if err != nil || first != second {
		return generation, err
	}
	term, otherGeneration, found, err := c.modelFinder.Inverse(quantifier, 0, raw[1])
	if err != nil || !found || term != bindings[1] {
		return generation, err
	}
	falsified, err := c.falsifiesRepeatedBinding(md, quantifier, raw[1])
	if err != nil || !falsified {
		return generation, err
	}
	_, retainedGeneration, found, err := c.modelFinder.Inverse(quantifier, 1, raw[1])
	if err != nil || !found {
		return generation, err
	}
	bindings[0] = term
	return max(retainedGeneration, otherGeneration), nil
}

func (c *SMTModelChecker) falsifiesRepeatedBinding(md *protomodel.ProtoModel, quantifier, value ast.ASTHandle) (falsified bool, result error) {
	m := c.manager
	ground, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ground)) }()
	if err = rewriter.InstantiateQuantifier(m, quantifier, []ast.ASTHandle{value, value}, ground); err != nil {
		return false, err
	}
	body, err := m.RefGet(ground)
	if err != nil {
		return false, err
	}
	evaluated, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&evaluated)) }()
	ok, err := md.Eval(body, evaluated, true)
	if err != nil || !ok {
		return false, err
	}
	answer, err := m.RefGet(evaluated)
	return err == nil && m.IsFalse(answer), err
}
