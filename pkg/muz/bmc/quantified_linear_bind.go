package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, qlinear::get_model substitutions.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
)

// QuantifiedLinearModelBindings evaluates a rule's indexed variable bindings
// at its selected rule ordinal, matching qlinear::get_model. The level is
// retained in the API because callers also track the selected trace level.
// The caller owns each nonempty reference.
func (c *Compiler) QuantifiedLinearModelBindings(rule *base.Rule, ruleIndex, level, width uint32, md *model.ModelCore, services model.ModelEvaluatorConfig) (result []ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || rule == nil || md == nil || md.Manager() != c.manager {
		return nil, ast.ErrASTHandle
	}
	m := c.manager
	owner := &quantifiedLinearBindingOwner{manager: m}
	defer func() {
		result, resultErr = finishQuantifiedLinearBindings(owner, resultErr)
	}()
	q, err := newQuantifiedLinear(m, width)
	owner.terms = q
	if err != nil {
		return nil, err
	}
	sub, err := rewriter.NewVarSubstitution(m, false)
	owner.substitution = sub
	if err != nil {
		return nil, err
	}
	predicate, err := rule.HeadDeclaration()
	if err != nil {
		return nil, err
	}
	bindings, err := q.bindings(rule, predicate, ruleIndex)
	if err != nil {
		return nil, err
	}
	index, err := q.hold(m.BitVectorNumeralRat(new(big.Rat).SetUint64(uint64(ruleIndex)), width))
	if err != nil {
		return nil, err
	}
	owner.outputs = make([]ast.ASTRef, len(bindings))
	result = owner.outputs
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
		// model::eval constructs a completion-disabled evaluator for each term.
		evaluator, cause := model.NewModelEvaluator(md, services)
		owner.evaluator = evaluator
		if cause != nil {
			return result, cause
		}
		if cause = evaluator.SetModelCompletion(false); cause != nil {
			return result, cause
		}
		result[i], cause = evaluator.EvaluateValue(term)
		if cause != nil {
			return result, cause
		}
		if cause = evaluator.Close(); cause != nil {
			return result, cause
		}
		owner.evaluator = nil
	}
	return result, nil
}
