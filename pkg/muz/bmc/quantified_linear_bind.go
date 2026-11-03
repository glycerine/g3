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
// at its selected derivation level. The caller owns each nonempty reference.
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
	evaluator, err := model.NewModelEvaluator(md, services)
	owner.evaluator = evaluator
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
	index, err := q.hold(m.BitVectorNumeralRat(new(big.Rat).SetUint64(uint64(level)), width))
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
		result[i], cause = evaluator.EvaluateValue(term)
		if cause != nil {
			return result, cause
		}
		value, cause := m.RefGet(result[i])
		if cause != nil {
			return result, cause
		}
		isValue, cause := m.IsValue(value)
		if cause != nil {
			return result, cause
		}
		if !isValue {
			// An uninterpreted function with no model entry evaluates to its
			// application, not a value. Keep the rule variable unbound, as the
			// linear path does for an absent model interpretation.
			sort, e := m.GetSort(term)
			if e != nil {
				return result, e
			}
			variable, e := m.MkVar(uint32(i), sort)
			if e != nil {
				return result, e
			}
			if e = m.SetRef(result[i], variable); e != nil {
				return result, e
			}
		}
	}
	return result, nil
}
