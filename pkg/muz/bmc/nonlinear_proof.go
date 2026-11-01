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
	if c == nil || c.manager == nil || rules == nil || md == nil || md.Manager() != c.manager {
		return result, ast.ErrASTHandle
	}
	m := c.manager
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, scope.Close())
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	evaluator, err := model.NewModelEvaluator(md, services)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	queryRef, err := c.CompileQuery(query, level)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&queryRef)) }()
	application, err := m.RefGet(queryRef)
	if err != nil {
		return result, err
	}
	var trace []*base.Rule
	defer func() {
		for i := len(trace) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, trace[i].Close())
		}
	}()
	if err = c.traceNonlinearRule(rules, evaluator, query, application, level, &trace, &result); err != nil {
		return result, err
	}
	return result, nil
}
