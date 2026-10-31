package api

// Source: src/solver/tactic2solver.cpp tactic2solver::translate and
// src/tactic/smtlogics/smt_tactic.cpp tactic translation.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func translateCallerTacticSolver(source *solver.TacticSolver, target *SMTContext, manager *ast.ASTManager, params *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result solver.Solver, resultErr error) {
	raw, err := source.TranslationTactic()
	if err != nil {
		return nil, err
	}
	targetTactic, err := tactic.TranslateWithContext(raw, manager, config.Params, func(leaf tactic.Tactic) (tactic.Tactic, error) {
		switch sourceTactic := leaf.(type) {
		case *smtcore.SMTTactic:
			return sourceTactic.TranslateWithConfig(manager, config)
		case *core.SimplifyTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals)
		default:
			return nil, fmt.Errorf("api: cross-context translation of this tactic is not yet supported")
		}
	})
	if err != nil {
		return nil, err
	}
	if err = targetTactic.IncRef(); err != nil {
		return nil, err
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
