package api

// Source: src/solver/tactic2solver.cpp tactic2solver::translate and
// src/tactic/smtlogics/smt_tactic.cpp tactic translation.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func translateSMTTacticSolver(source *solver.TacticSolver, target *SMTContext, manager *ast.ASTManager, params *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result solver.Solver, resultErr error) {
	raw, err := source.TranslationTactic()
	if err != nil {
		return nil, err
	}
	sourceTactic, ok := raw.(*smtcore.SMTTactic)
	if !ok {
		return nil, fmt.Errorf("api: cross-context translation of this tactic is not yet supported")
	}
	targetTactic, err := sourceTactic.TranslateWithConfig(manager, config)
	if err != nil {
		return nil, err
	}
	if err = targetTactic.IncRef(); err != nil {
		return nil, errors.Join(err, targetTactic.Close())
	}
	defer func() {
		resultErr = errors.Join(resultErr, targetTactic.DecRef())
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	template, err := target.command.NewSolverFromTactic(targetTactic)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, template.Close())
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	targetSolver, ok := template.(*solver.TacticSolver)
	if !ok {
		return nil, fmt.Errorf("api: unsupported target tactic solver")
	}
	return source.TranslateWithTarget(targetSolver, params)
}
