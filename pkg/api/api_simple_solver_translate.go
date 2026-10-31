package api

// Source: src/api/api_solver.cpp Z3_solver_translate.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/smtcore"
)

// Translate copies a direct SMT solver into target. The returned solver owns
// its translated state and must be closed before target is closed.
func (s *APISimpleSolver) Translate(target *SMTContext) (result *APISimpleSolver, resultErr error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if err := target.open(); err != nil {
		return nil, err
	}
	direct, ok := s.solver.(*smtcore.SMTSolver)
	if !ok {
		return nil, fmt.Errorf("api: translation of this solver implementation is not yet supported")
	}
	manager, err := target.Manager()
	if err != nil {
		return nil, err
	}
	config, err := target.command.SolverTranslationConfig()
	if err != nil {
		return nil, err
	}
	sourceGlobals, err := s.owner.command.GlobalParameters()
	if err != nil {
		return nil, err
	}
	params, err := s.solver.Params()
	if err != nil {
		return nil, err
	}
	translatedParams, err := sourceGlobals.Context.TranslateTo(params, config.Params)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, config.Params.Del(&translatedParams))
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	translated, err := direct.TranslateWithConfig(manager, &translatedParams, config)
	if err != nil {
		return nil, err
	}
	return &APISimpleSolver{owner: target.smtContextState, solver: translated}, nil
}
