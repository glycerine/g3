package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, qlinear::get_model substitutions.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
)

// QuantifiedLinearModelBindings evaluates a rule's indexed variable bindings
// at its selected derivation level. The caller owns each nonempty reference.
func (c *Compiler) QuantifiedLinearModelBindings(rule *base.Rule, ruleIndex, level, width uint32, md *model.ModelCore, services model.ModelEvaluatorConfig) (result []ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || rule == nil || md == nil || md.Manager() != c.manager {
		return nil, ast.ErrASTHandle
	}
	m := c.manager
	defer func() {
		if resultErr != nil {
			for i := len(result) - 1; i >= 0; i-- {
				if result[i] != (ast.ASTRef{}) {
					resultErr = errors.Join(resultErr, m.DelRef(&result[i]))
				}
			}
			result = nil
		}
	}()
	q, err := newQuantifiedLinear(m, width)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, q.close()) }()
	evaluator, err := model.NewModelEvaluator(md, services)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	sub, err := rewriter.NewVarSubstitution(m, false)
	if err != nil {
		if sub != nil {
			err = errors.Join(err, sub.Close())
		}
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	predicate, err := rule.HeadDeclaration()
	if err != nil {
		return nil, err
	}
	bindings, err := q.bindings(rule, predicate, ruleIndex)
	if err != nil {
		return nil, err
	}
	index, err := q.hold(m.BitVectorNumeralRat(new(big.Rat).SetUint64(uint64(level)), width))
	if err != nil {
		return nil, err
	}
	result = make([]ast.ASTRef, len(bindings))
	for i, binding := range bindings {
		if binding.IsNull() {
			continue
		}
		instantiated, cause := m.NewRef(ast.ASTHandle{})
		if cause != nil {
			return result, cause
		}
		q.refs = append(q.refs, instantiated)
		if cause = sub.Substitute(binding, []ast.ASTHandle{index}, instantiated); cause != nil {
			return result, cause
		}
		term, cause := m.RefGet(instantiated)
		if cause != nil {
			return result, cause
		}
		result[i], cause = evaluator.EvaluateValue(term)
		if cause != nil {
			return result, cause
		}
	}
	return result, nil
}
