package api

// Source: src/api/api_solver.cpp:206-248.

import (
	"errors"

	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// collectDescriptions collects the built solver's parameter descriptions
// and the context's solver parameters.
func (s *APISimpleSolver) collectDescriptions() (*ParameterDescriptions, error) {
	return s.getSolverDescriptions(s.solver.CollectParamDescrs)
}

func (s *APISimpleSolver) getSolverDescriptions(collect func(*uparams.ParamDescrs) error) (*ParameterDescriptions, error) {
	globals, err := s.owner.command.GlobalParameters()
	if err != nil {
		return nil, err
	}
	value, err := uparams.NewParamDescrs(globals.Context.Symbols(), uparams.ParamDescrsConfig{Budget: globals.Context.Budget()})
	if err != nil {
		return nil, err
	}
	if err = collect(value); err != nil {
		return nil, errors.Join(err, value.Close())
	}
	if err = smtcore.CollectSolverParamDescriptions(value); err != nil {
		return nil, errors.Join(err, value.Close())
	}
	return &ParameterDescriptions{owner: s.owner, value: value, symbols: globals.Context.Symbols()}, nil
}

// ParameterDescriptions is Z3_solver_get_param_descrs: a solver that was
// not built yet is built for the descriptions and dropped again.
func (s *APISimpleSolver) ParameterDescriptions() (result *ParameterDescriptions, resultErr error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	initialized := s.solver != nil
	if err := s.ensure(); err != nil {
		return nil, err
	}
	if !initialized {
		defer func() {
			if err := s.dropSolver(); err != nil {
				resultErr = errors.Join(resultErr, err)
				if result != nil {
					resultErr = errors.Join(resultErr, result.Close())
					result = nil
				}
			}
		}()
	}
	return s.owner.saveDescriptions(s.collectDescriptions())
}

// updateSolverParams is updt_params of the solver's current parameters
// with parameters appended.
func (s *APISimpleSolver) updateSolverParams(parameters *ParameterSet) (resultErr error) {
	merged, err := parameters.params.NewValue()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.params.Del(&merged)) }()
	current, err := s.solver.Params()
	if err != nil {
		return err
	}
	if err = parameters.params.Append(&merged, current); err != nil {
		return err
	}
	if err = parameters.params.Append(&merged, &parameters.value); err != nil {
		return err
	}
	return s.solver.UpdateParams(&merged)
}

// SetParameters is Z3_solver_set_params: smt.logic becomes the handle's
// logic; a built solver validates and takes the parameters at once; the
// handle keeps them (m_params) for the solver init_solver builds later.
func (s *APISimpleSolver) SetParameters(parameters *ParameterSet) error {
	if err := s.open(); err != nil {
		return err
	}
	if err := parameters.open(); err != nil {
		return err
	}
	if parameters.owner != s.owner {
		return ErrSMTContextOwner
	}
	logicName := "smt.logic"
	logic, err := parameters.params.GetSymbolName(&parameters.value, &logicName, usymbol.Symbol{})
	if err != nil {
		return err
	}
	if !logic.IsNull() {
		if s.logic, err = parameters.params.Symbols().String(logic); err != nil {
			return err
		}
	}
	if s.solver != nil {
		name := "model"
		oldModels := true
		if s.explicitReady {
			oldModels, err = parameters.params.GetBoolName(&s.explicitParams, &name, true)
			if err != nil {
				return err
			}
		}
		models, err := parameters.params.GetBoolName(&parameters.value, &name, true)
		if err != nil {
			return err
		}
		if oldModels != models {
			if err = s.solver.SetProduceModels(models); err != nil {
				return err
			}
		}
		if err = s.validateExplicit(parameters); err != nil {
			return err
		}
		if err = s.updateSolverParams(parameters); err != nil {
			return err
		}
	}
	if !s.explicitReady {
		s.explicitParams, err = parameters.params.NewValue()
		if err != nil {
			return err
		}
		s.explicitContext = parameters.params
		s.explicitReady = true
	}
	return parameters.params.Append(&s.explicitParams, &parameters.value)
}

// validateExplicit keeps the full catalog authoritative for diagnostics and
// tactic-specific options. Most incremental callers set only SMT parameters.
func (s *APISimpleSolver) validateExplicit(parameters *ParameterSet) error {
	if combined, ok := s.solver.(*solver.CombinedSolver); ok && combined.HasDeferredTactic() {
		partial, err := s.getSolverDescriptions(combined.CollectIncrementalParamDescrs)
		if err != nil {
			return err
		}
		covered, err := parameters.params.AllDescribed(&parameters.value, partial.value)
		if err == nil && covered {
			err = parameters.Validate(partial)
		}
		if closeErr := partial.Close(); err != nil || closeErr != nil || covered {
			return errors.Join(err, closeErr)
		}
	}
	descriptions, err := s.collectDescriptions()
	if err != nil {
		return err
	}
	return errors.Join(parameters.Validate(descriptions), descriptions.Close())
}
