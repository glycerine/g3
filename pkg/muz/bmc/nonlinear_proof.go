package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, nonlinear::get_proof/get_model.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
)

// NonlinearProof constructs the model-selected proof before applying the
// fixedpoint preprocessing proof converter. All selected rules must carry the
// source proofs needed by nonlinear::get_proof. The caller owns the result.
func (c *Compiler) NonlinearProof(rules *base.RuleSet, query ast.ASTHandle, level uint32, md *model.ModelCore, services model.ModelEvaluatorConfig) (result ast.ASTRef, resultErr error) {
	trace, result, err := c.nonlinearModel(rules, query, level, md, services, false)
	return result, disposeRuleOwners(c.manager, trace, err)
}

// NonlinearModel is nonlinear::get_model: the model is printed (verbose 2),
// then one get_proof pass fills m_rule_trace and builds the proof. The
// caller owns both results.
func (c *Compiler) NonlinearModel(rules *base.RuleSet, query ast.ASTHandle, level uint32, md *model.ModelCore, services model.ModelEvaluatorConfig) ([]*base.Rule, ast.ASTRef, error) {
	return c.nonlinearModel(rules, query, level, md, services, true)
}

func (c *Compiler) nonlinearModel(rules *base.RuleSet, query ast.ASTHandle, level uint32, md *model.ModelCore, services model.ModelEvaluatorConfig, verbose bool) (trace []*base.Rule, result ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || rules == nil || md == nil || md.Manager() != c.manager {
		return nil, result, ast.ErrASTHandle
	}
	m := c.manager
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return trace, result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, scope.Close())
		if resultErr != nil && result != (ast.ASTRef{}) {
			// Suppress publication only after transferring the concrete proof
			// token to a retryable cleanup receipt on native refusal.
			resultErr = disposeASTRefs(m, []ast.ASTRef{result}, resultErr)
			result = ast.ASTRef{}
		}
	}()
	evaluator, err := model.NewModelEvaluator(md, services)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return trace, result, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	queryRef, err := c.CompileQuery(query, level)
	if err != nil {
		return trace, result, err
	}
	defer func() { resultErr = disposeASTRefs(m, []ast.ASTRef{queryRef}, resultErr) }()
	application, err := m.RefGet(queryRef)
	if err != nil {
		return trace, result, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = disposeRuleOwners(m, trace, resultErr)
			trace = nil
		}
	}()
	if verbose {
		// nonlinear::get_model prints the model before get_proof.
		if err = verboseModel(2, md, services); err != nil {
			return trace, result, err
		}
	}
	if err = c.traceNonlinearRule(rules, evaluator, query, application, level, &trace, &result); err != nil {
		return trace, result, err
	}
	return trace, result, nil
}
