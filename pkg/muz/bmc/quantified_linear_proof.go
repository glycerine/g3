package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, qlinear::get_model proof assembly.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

// QuantifiedLinearProof constructs the unconverted proof for the selected
// rule trace. The caller owns the returned proof reference.
func (c *Compiler) QuantifiedLinearProof(rules *base.RuleSet, query ast.ASTHandle, width uint32, md *model.ModelCore, services model.ModelEvaluatorConfig) (result ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || rules == nil || md == nil || md.Manager() != c.manager || services.Context == nil || services.Globals == nil {
		return result, ast.ErrASTHandle
	}
	m := c.manager
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	rm, err := rules.RuleManager()
	if err != nil {
		return result, err
	}
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, scope.Close()) }()
	normalizer := transforms.NewRuleInlinerPluginWithServices(m, rm, nil, true, true, services.Context, services.Globals, 0)
	defer func() { resultErr = errors.Join(resultErr, normalizer.Close()) }()
	steps, err := c.quantifiedLinearSteps(rules, query, width, md, services)
	if err != nil {
		return result, err
	}
	defer func() {
		for i := len(steps) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, steps[i].rule.Close())
		}
	}()
	var previous *base.Rule
	defer func() {
		if previous != nil {
			resultErr = errors.Join(resultErr, previous.Close())
		}
	}()
	for _, step := range steps {
		bindings, cause := c.QuantifiedLinearModelBindings(step.rule, step.index, step.level, width, md, services)
		if cause != nil {
			return result, cause
		}
		// The accumulated rule's first body atom resolves with the next
		// rule's head; the source qlinear path reverses these positions.
		position := ast.ASTProofPosition{First: 1, Second: 0}
		next, proof, cause := c.linearProofStepWithBindings(rm, step.rule, bindings, previous, result, normalizer, position)
		if cause != nil {
			if next != nil {
				cause = errors.Join(cause, next.Close())
			}
			if proof != (ast.ASTRef{}) {
				cause = errors.Join(cause, m.DelRef(&proof))
			}
			return result, cause
		}
		if previous != nil {
			if cause = previous.Close(); cause != nil {
				return result, errors.Join(cause, next.Close(), m.DelRef(&proof))
			}
		}
		previous = next
		if result != (ast.ASTRef{}) {
			if cause = m.DelRef(&result); cause != nil {
				return result, errors.Join(cause, m.DelRef(&proof))
			}
		}
		result = proof
	}
	if result == (ast.ASTRef{}) {
		return result, ast.ErrASTDomain
	}
	return result, nil
}
