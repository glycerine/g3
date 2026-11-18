package opt

// Source: src/opt/maxres.cpp update_assignment upper-bound/model retention.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (e *maxResEngine) updateMUSModel(result *ArithmeticResult) error {
	if e == nil || e.mus == nil {
		return errMaxResState
	}
	var candidate *model.ModelCore
	// rational w; the mus's best model weight (0 for the sat core)
	weight := numeral.NewRational()
	defer func() { e.noteRat(e.q().Del(&weight)) }()
	var err error
	if e.config.satCoreEnabled {
		candidate, err = e.solver.solver().AcquireModel()
	} else {
		candidate, err = e.mus.BestModel(&weight)
	}
	if err != nil || candidate == nil {
		return err
	}
	if e.cmp(&weight, &e.upper) < 0 {
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
	// maxsmt_solver_base::init evaluates the initial model without
	// completion; update_assignment's is_true completes it.
	var cost, satisfied numeral.Rational
	var assignment []bool
	var err error
	if e.initialCandidate && e.initEvaluated {
		cost, satisfied, assignment = e.initCost, e.initSatisfied, e.initAssignment
		e.initCost, e.initSatisfied, e.initAssignment, e.initEvaluated = numeral.Rational{}, numeral.Rational{}, nil, false
	} else {
		cost, satisfied, assignment, err = e.solver.evaluateSoftAssignment(e.objective, md, !e.initialCandidate)
	}
	defer func() { e.noteRat(errors.Join(e.q().Del(&cost), e.q().Del(&satisfied))) }()
	if err != nil {
		return errors.Join(fmt.Errorf("maxres candidate objective: %w", err), md.DecRef())
	}
	e.stats.Candidates++
	// Only update_assignment counts the correction set (is_false, with
	// model completion); maxsmt_solver_base::init never evaluates m_asms
	// in the base model, which opt::context shares as m_model
	// (AUDIT.md FUZZ_A-F11).
	var correction []ast.ASTHandle
	if !e.initialCandidate {
		correction, err = e.correctionSet(md)
		if err != nil {
			return errors.Join(fmt.Errorf("maxres candidate correction: %w", err), md.DecRef())
		}
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
	if e.bestSet && e.cmp(&cost, &e.upper) > 0 {
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
	if err = setSoftObjectiveValue(result, e.index, &cost, &satisfied, assignment); err != nil {
		return err
	}
	if err = e.q().Copy(&e.upper, &cost); err != nil {
		return err
	}
	e.bestSet = true
	e.stats.Improvements++
	// update_assignment: m_upper = upper; trace(); add_upper_bound_block()
	// (AUDIT.md FUZZ-F5). maxsmt_solver_base::init does not trace.
	if !e.initialCandidate {
		if err = e.trace(); err != nil {
			return err
		}
	}
	return e.assertUpperBound(&satisfied)
}
