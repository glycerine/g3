package cmd_context

// Source: src/opt/opt_pareto.cpp GIA dominance loop and
// src/opt/opt_context.cpp Pareto comparison/model callbacks.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type paretoModelValue struct {
	measure    big.Rat
	softCost   big.Rat
	comparable bool
}

func (c *Context) evaluateParetoModel(md *model.ModelCore) (values []paretoModelValue, resultErr error) {
	if md == nil {
		return nil, ast.ErrASTHandle
	}
	evaluator, err := model.NewModelEvaluator(md, c.config.ModelEvaluator)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	// C++ model::eval defaults model_completion to false in opt_context::mk_cmp.
	if err = evaluator.SetModelCompletion(false); err != nil {
		return nil, err
	}
	values = make([]paretoModelValue, len(c.objectives))
	for i := range c.objectives {
		objective := &c.objectives[i]
		if objective.soft {
			values[i].comparable = true
			// from_fmls' adjust_value offset (is_maxsat moves negative
			// weights onto negated terms).
			values[i].softCost.Set(&objective.softOffset)
			for j := range objective.softTerms {
				term := &objective.softTerms[j]
				ref, e := evaluator.EvaluateValue(term.expression)
				if e != nil {
					return nil, e
				}
				resolved, e := c.manager.RefGet(ref)
				truth := e == nil && c.manager.IsTrue(resolved)
				e = errors.Join(e, c.manager.DelRef(&ref))
				if e != nil {
					return nil, e
				}
				weight := new(big.Rat).Set(&term.weight)
				if weight.Sign() < 0 {
					values[i].softCost.Add(&values[i].softCost, weight)
					weight.Neg(weight)
					truth = !truth
				}
				if truth {
					values[i].measure.Add(&values[i].measure, weight)
				} else {
					values[i].softCost.Add(&values[i].softCost, weight)
				}
			}
			continue
		}
		ref, e := evaluator.EvaluateValue(c.objectives[i].expression)
		if e != nil {
			return nil, e
		}
		h, e := c.manager.RefGet(ref)
		if e == nil {
			var text string
			var matched bool
			if c.objectives[i].bitVector {
				text, matched, e = c.manager.BitVectorNumeralText(h)
			} else {
				text, _, matched, e = c.manager.ArithmeticNumeralText(h)
			}
			if e == nil {
				if !matched {
					values[i].comparable = false
				} else if _, parsed := values[i].measure.SetString(text); !parsed {
					e = ast.ErrASTDomain
				} else {
					values[i].comparable = true
				}
			}
		}
		e = errors.Join(e, c.manager.DelRef(&ref))
		if e != nil {
			return nil, e
		}
	}
	return values, nil
}

// makeParetoFormula returns an owned reference to the formula that
// buildParetoFormula hands to its consumer.
func (c *Context) makeParetoFormula(md *model.ModelCore, dominates bool) (result ast.ASTRef, resultErr error) {
	resultErr = c.buildParetoFormula(md, dominates, func(formula ast.ASTHandle) error {
		var err error
		result, err = c.manager.NewRef(formula)
		return err
	})
	return result, resultErr
}

// buildParetoFormula follows pareto_base::mk_dominates and
// mk_not_dominated_by. Dominance pushes each GE then GT comparison and one
// flat conjunction; the persistent blocker invokes only LE comparisons. The
// consumer (m_solver->assert_expr) runs while the comparison vectors are
// still alive; on return they die in reverse declaration order (fmls, then
// gt, each front to back), and fml last.
func (c *Context) buildParetoFormula(md *model.ModelCore, dominates bool, use func(ast.ASTHandle) error) (resultErr error) {
	if md == nil {
		return ast.ErrASTHandle
	}
	// expr_ref fml(m); expr_ref_vector gt(m), fmls(m) (le(m) for the blocker).
	var fml ast.ASTRef
	var gt, fmls []ast.ASTRef
	defer func() {
		for i := range fmls {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&fmls[i]))
		}
		for i := range gt {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&gt[i]))
		}
		resultErr = errors.Join(resultErr, c.manager.DelRef(&fml))
	}()
	handles := func(refs []ast.ASTRef) ([]ast.ASTHandle, error) {
		out := make([]ast.ASTHandle, len(refs))
		for i := range refs {
			h, err := c.manager.RefGet(refs[i])
			if err != nil {
				return nil, err
			}
			out[i] = h
		}
		return out, nil
	}
	for i := range c.objectives {
		kind := OptimizationModelLE
		if dominates {
			kind = OptimizationModelGE
		}
		ref, err := c.OptimizationCompareModel(uint32(i), md, kind)
		if err != nil {
			return err
		}
		fmls = append(fmls, ref)
		if dominates {
			ref, err = c.OptimizationCompareModel(uint32(i), md, OptimizationModelGT)
			if err != nil {
				return err
			}
			gt = append(gt, ref)
		}
	}
	var formula ast.ASTHandle
	if dominates {
		args, err := handles(gt)
		if err != nil {
			return err
		}
		h, err := c.manager.MkOrArgs(args)
		if err != nil {
			return err
		}
		ref, err := c.manager.NewRef(h)
		if err != nil {
			return err
		}
		fmls = append(fmls, ref)
		if args, err = handles(fmls); err != nil {
			return err
		}
		if formula, err = c.manager.MkAndArgs(args); err != nil {
			return err
		}
	} else {
		args, err := handles(fmls)
		if err != nil {
			return err
		}
		conj, err := c.manager.MkAndArgs(args)
		if err != nil {
			return err
		}
		if formula, err = c.manager.MkNot(conj); err != nil {
			return err
		}
	}
	var err error
	if fml, err = c.manager.NewRef(formula); err != nil {
		return err
	}
	return use(formula)
}

func (c *Context) assertParetoFormula(backend *smtcore.SMTSolver, md *model.ModelCore, dominates bool) (resultErr error) {
	if backend == nil {
		return ast.ErrASTHandle
	}
	return c.buildParetoFormula(md, dominates, func(formula ast.ASTHandle) error {
		if err := c.paretoFormulaDiagnostics(formula, md, dominates); err != nil {
			return err
		}
		return backend.Assert(formula)
	})
}

// finishParetoSearch is set_pareto(nullptr): the gia_pareto engine ends,
// but its opt_solver (and normalize's state) stays opt::context's
// m_opt_solver until the next init_solver replaces it.
func (c *Context) finishParetoSearch() {
	c.optimizationParetoActive = false
}

// releaseParetoInitialModel drops opt::context::m_model when it still
// holds optimize()'s initial-check model (m_model.reset() in clear_state,
// or yield's assignment from the pareto engine).
func (c *Context) releaseParetoInitialModel() error {
	md := c.optimizationParetoInitialModel
	if md == nil {
		return nil
	}
	c.optimizationParetoInitialModel = nil
	return md.DecRef()
}

func (c *Context) closeParetoBackend() error {
	c.optimizationParetoActive = false
	if c.optimizationParetoBackend != nil {
		if err := c.optimizationParetoBackend.Close(); err != nil {
			return err
		}
		c.optimizationParetoBackend = nil
	}
	if state := c.optimizationParetoPurification; state != nil {
		if state.converter != nil {
			if err := state.converter.DecRef(); err != nil {
				return err
			}
			state.converter = nil
		}
		if err := state.purifier.Close(); err != nil {
			return err
		}
		c.optimizationParetoPurification = nil
	}
	return nil
}

func (c *Context) newParetoBackend(assumptions []ast.ASTHandle) (backend *smtcore.SMTSolver, resultErr error) {
	// gia_pareto runs on opt::context's m_solver, the opt_solver that
	// init_solver built; take it over when this check created one.
	var err error
	if c.preparedOptimizer != nil {
		backend = c.preparedOptimizer.DetachBackend()
	}
	if backend == nil {
		backend, err = smtcore.NewOptimizationSMTSolver(c.manager, &c.params, c.logic, c.config)
		if err != nil {
			return nil, err
		}
	}
	published := false
	ownedBackend := backend
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, ownedBackend.Close())
		}
	}()
	if err = backend.SetProduceModels(true); err != nil {
		return nil, err
	}
	for i := range c.objectives {
		if c.objectives[i].soft {
			if err = backend.EnsurePseudoBooleanTheory(); err != nil {
				return nil, err
			}
			break
		}
	}
	hard := make([]ast.ASTHandle, len(c.assertions))
	for i := range c.assertions {
		hard[i] = c.assertions[i].expression
	}
	if state := c.optimizationParetoPurification; state != nil {
		hard = state.hard
	}
	for _, assertion := range hard {
		if err = backend.Assert(assertion); err != nil {
			return nil, err
		}
	}
	for _, assumption := range assumptions {
		if err = backend.Assert(assumption); err != nil {
			return nil, err
		}
	}
	published = true
	return backend, nil
}

func (c *Context) checkOptimizePareto(assumptions []ast.ASTHandle, continuation bool) (result *opt.ArithmeticResult, resultErr error) {
	// optimize() with no m_pareto engine runs init_solver and normalize;
	// otherwise it resumes execute_pareto.
	fresh := !continuation || !c.optimizationParetoActive
	if fresh {
		if err := c.closeParetoBackend(); err != nil {
			return nil, err
		}
		if err := c.prepareParetoPurification(); err != nil {
			return nil, err
		}
		backend, err := c.newParetoBackend(assumptions)
		if err != nil {
			return nil, errors.Join(err, c.closeParetoBackend())
		}
		c.optimizationParetoBackend = backend
	}
	original := c.objectives
	c.objectives = c.optimizationParetoPurification.objectives
	defer func() { c.objectives = original }()
	c.optimizationParetoActive = true
	backend := c.optimizationParetoBackend
	status, err := backend.CheckSat(nil)
	if err != nil {
		return nil, err
	}
	unknownReason := ""
	if status == lbool.Undef {
		unknownReason, err = backend.ReasonUnknown()
		if err != nil {
			return nil, err
		}
	}
	if fresh && status != lbool.False {
		// optimize(): s.get_model(m_model) after the initial check.
		if err = c.releaseParetoInitialModel(); err != nil {
			return nil, err
		}
		if c.optimizationParetoInitialModel, err = backend.AcquireModel(); err != nil {
			return nil, err
		}
	}
	if status == lbool.True && fresh {
		// optimize(): after a sat initial check, optsmt::setup forces the
		// base level with an empty scoped push, and execute_pareto's
		// gia_pareto then runs its own check_sat.
		if err = backend.Push(); err != nil {
			return nil, err
		}
		if err = backend.Pop(1); err != nil {
			return nil, err
		}
		if status, err = backend.CheckSat(nil); err != nil {
			return nil, err
		}
		if status == lbool.Undef {
			if unknownReason, err = backend.ReasonUnknown(); err != nil {
				return nil, err
			}
		}
	}
	if status != lbool.True {
		c.finishParetoSearch()
		return &opt.ArithmeticResult{Status: status, ReasonUnknown: unknownReason, Values: make([]opt.ObjectiveValue, len(c.objectives)), Optimal: status == lbool.False}, nil
	}
	canceled, cancelErr := c.manager.Canceled()
	if cancelErr != nil {
		return nil, cancelErr
	}
	if canceled {
		c.finishParetoSearch()
		return &opt.ArithmeticResult{Status: lbool.Undef, Values: make([]opt.ObjectiveValue, len(c.objectives))}, nil
	}
	var best *model.ModelCore
	defer func() {
		if resultErr != nil && best != nil {
			resultErr = errors.Join(resultErr, best.DecRef())
		}
	}()
	// PushCore clears the Go solver's published result. The C++ scoped push
	// leaves get_model available, so take an owned snapshot before pushing.
	// It represents the same model the first iteration reads in gia_pareto.
	best, err = backend.AcquireModel()
	if err != nil {
		return nil, err
	}
	if best == nil {
		return nil, ast.ErrASTHandle
	}
	labels, err := backend.Labels()
	if err != nil {
		return nil, err
	}

	if err = backend.Push(); err != nil {
		return nil, err
	}
	pushed := true
	defer func() {
		if pushed {
			resultErr = errors.Join(resultErr, backend.Pop(1))
		}
	}()
	firstIteration := true
	for status == lbool.True {
		if !firstIteration {
			canceled, cancelErr = c.manager.Canceled()
			if cancelErr != nil {
				return nil, cancelErr
			}
			if canceled {
				status = lbool.Undef
				break
			}
			candidate, acquireErr := backend.AcquireModel()
			if acquireErr != nil {
				return nil, acquireErr
			}
			if candidate == nil {
				return nil, ast.ErrASTHandle
			}
			candidateLabels, labelsErr := backend.Labels()
			if labelsErr != nil {
				return nil, errors.Join(labelsErr, candidate.DecRef())
			}

			if err = best.DecRef(); err != nil {
				return nil, errors.Join(err, candidate.DecRef())
			}
			best, labels = candidate, candidateLabels
		}
		firstIteration = false
		if err = c.paretoModelDiagnostic(best); err != nil {
			return nil, err
		}
		if err = c.assertParetoFormula(backend, best, true); err != nil {
			return nil, err
		}
		status, err = backend.CheckSat(nil)
		if err != nil {
			return nil, err
		}
		if status == lbool.Undef {
			unknownReason, err = backend.ReasonUnknown()
			if err != nil {
				return nil, err
			}
		}
	}
	if err = backend.Pop(1); err != nil {
		return nil, err
	}
	pushed = false
	if status != lbool.False {
		fixed, err := c.filterParetoModel(best)
		if err != nil {
			return nil, err
		}
		if err = best.DecRef(); err != nil {
			return nil, errors.Join(err, fixed.DecRef())
		}
		best = fixed
		c.finishParetoSearch()
		result = &opt.ArithmeticResult{Status: status, ReasonUnknown: unknownReason, Values: make([]opt.ObjectiveValue, len(c.objectives)), Model: best, Labels: labels}
		best = nil
		return result, nil
	}
	if err = c.assertParetoFormula(backend, best, false); err != nil {
		return nil, err
	}
	// execute_pareto's yield: m_model takes the pareto engine's model.
	if err = c.releaseParetoInitialModel(); err != nil {
		return nil, err
	}
	values, err := c.evaluateParetoModel(best)
	if err != nil {
		return nil, err
	}

	fixed, err := c.filterParetoModel(best)
	if err != nil {
		return nil, err
	}
	if err = best.DecRef(); err != nil {
		return nil, errors.Join(err, fixed.DecRef())
	}
	best = fixed
	result = &opt.ArithmeticResult{Status: lbool.True, Values: make([]opt.ObjectiveValue, len(c.objectives)), Model: best, Labels: labels, Optimal: true}
	best = nil
	for i := range values {
		if c.objectives[i].soft {
			result.Values[i].SoftCost.Set(&values[i].softCost)
			result.Values[i].Soft = true
			continue
		}
		if c.objectives[i].bitVector {
			result.Values[i].SoftCost.Set(&values[i].measure)
			if c.objectives[i].maximize {
				maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(c.objectives[i].bitWidth)), big.NewInt(1))
				result.Values[i].SoftCost.Sub(new(big.Rat).SetInt(maximum), &values[i].measure)
			}
			continue
		}
		if !values[i].comparable {
			result.Values[i].ArithmeticOpenBounds = true
			continue
		}
		finite := simplex.RationalInfinitesimal(&values[i].measure)
		result.Values[i] = opt.ObjectiveValue{
			Arithmetic:           smtcore.SMTArithmeticObjectiveValue{Finite: finite},
			ArithmeticModel:      smtcore.SMTArithmeticObjectiveValue{Finite: finite.Clone()},
			ArithmeticModelValid: true,
		}
	}
	return result, nil
}
