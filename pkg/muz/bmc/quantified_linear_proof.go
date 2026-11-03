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
			owned := result
			result = ast.ASTRef{}
			resultErr = disposeASTRefs(m, []ast.ASTRef{owned}, resultErr)
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
		owners := make([]*base.Rule, len(steps))
		for i := range steps {
			owners[i] = steps[i].rule
		}
		resultErr = disposeRuleOwners(m, owners, resultErr)
	}()
	var previous *base.Rule
	defer func() {
		if previous != nil {
			resultErr = disposeRuleOwners(m, []*base.Rule{previous}, resultErr)
			previous = nil
		}
	}()
	for _, step := range steps {
		bindings, cause := c.QuantifiedLinearModelBindings(step.rule, step.index, step.level, width, md, services)
		if cause != nil {
			return result, cause
		}
		// The accumulated rule's first body atom resolves with the next
		// rule's head: body position one and head position zero.
		position := ast.ASTProofPosition{First: 1, Second: 0}
		next, proof, cause := c.linearProofStepWithBindings(rm, step.rule, bindings, previous, result, normalizer, position)
		if cause != nil {
			_, _, cause = advanceQuantifiedLinearProofStep(m, nil, next, ast.ASTRef{}, proof, cause)
			return result, cause
		}
		oldPrevious, oldResult := previous, result
		previous, result = nil, ast.ASTRef{}
		previous, result, cause = advanceQuantifiedLinearProofStep(m, oldPrevious, next, oldResult, proof, nil)
		if cause != nil {
			return result, cause
		}
	}
	if result == (ast.ASTRef{}) {
		return result, ast.ErrASTDomain
	}
	return result, nil
}
