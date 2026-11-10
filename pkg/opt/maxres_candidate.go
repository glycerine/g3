package opt

// Source: src/opt/maxres.cpp update_assignment upper-bound/model retention.

import "errors"
import "fmt"
import "math/big"

import "github.com/glycerine/g3/pkg/model"

func (e *maxResEngine) updateMUSModel(result *ArithmeticResult) error {
	if e == nil || e.mus == nil {
		return errMaxResState
	}
	var candidate *model.ModelCore
	var weight big.Rat
	var err error
	if e.config.satCoreEnabled {
		candidate, err = e.solver.solver().AcquireModel()
	} else {
		var value *big.Rat
		candidate, value, err = e.mus.BestModel()
		if value != nil {
			weight.Set(value)
		}
	}
	if err != nil || candidate == nil {
		return err
	}
	if weight.Cmp(&e.upper) < 0 {
		return e.updateCandidateModel(result, candidate)
	}
	return candidate.DecRef()
}

func (e *maxResEngine) updateCandidate(result *ArithmeticResult) error {
	md, err := e.solver.solver().AcquireModel()
	if err != nil {
		return fmt.Errorf("maxres candidate model: %w", err)
	}
	if md == nil {
		return errMaxResState
	}
	return e.updateCandidateModel(result, md)
}

// updateCandidateModel consumes one model reference, matching the source
// maxsmt_solver_base model owner passed into MaxRes initialization.
func (e *maxResEngine) updateCandidateModel(result *ArithmeticResult, md *model.ModelCore) error {
	if md == nil {
		return errMaxResState
	}
	cost, satisfied, assignment, err := e.solver.evaluateSoftDetailed(e.objective, md)
	if err != nil {
		return errors.Join(fmt.Errorf("maxres candidate objective: %w", err), md.DecRef())
	}
	e.stats.Candidates++
	correction, err := e.correctionSet(md)
	if err != nil {
		return errors.Join(fmt.Errorf("maxres candidate correction: %w", err), md.DecRef())
	}
	e.currentCorrection = append(e.currentCorrection[:0], correction...)
	correctionSize := len(correction)
	// init_local leaves m_csmodel null: maxsmt_solver_base::init's model
	// sets the bounds, and only later update_assignment calls seed it.
	if !e.initialCandidate && (e.correctionModel == nil || correctionSize < e.correctionSize) {
		if err = md.IncRef(); err != nil {
			return errors.Join(err, md.DecRef())
		}
		if e.correctionModel != nil {
			if err = e.correctionModel.DecRef(); err != nil {
				return errors.Join(err, md.DecRef())
			}
		}
		e.correctionModel = md
		e.correctionSize = correctionSize
	}
	if e.bestSet && cost.Cmp(&e.upper) > 0 {
		return md.DecRef()
	}
	// update_assignment: if (!m_c.verify_model(m_index, mdl, upper)) return;
	// The initial candidate is maxsmt_solver_base::init's model, which does
	// not go through update_assignment.
	if !e.initialCandidate {
		verified, err := e.solver.verifyModel(e.index, md, &cost)
		if err != nil || !verified {
			return errors.Join(err, md.DecRef())
		}
	}
	if result.Model != nil {
		if err = result.Model.DecRef(); err != nil {
			return errors.Join(err, md.DecRef())
		}
	}
	result.Model = md
	if err = e.solver.captureResultLabels(result); err != nil {
		return err
	}
	setSoftObjectiveValue(result, e.index, cost, satisfied, assignment)
	e.upper.Set(&cost)
	e.bestSet = true
	e.stats.Improvements++
	return e.assertUpperBound(&satisfied)
}
