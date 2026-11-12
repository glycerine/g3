package cmd_context

// Source: src/opt/opt_context.cpp normalize/simplify_fmls.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/bv"
	tacticcore "github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type optimizationSimplifiedFormulas struct {
	decoded      opt.DecodedFormulaSet
	refs         []ast.ASTRef
	held         []ast.ASTRef // from_fmls's objective terms and hard constraints
	model        tactic.ModelConverter
	manager      *ast.ASTManager
	inconsistent bool
	closed       bool
}

func (f *optimizationSimplifiedFormulas) Close() (result error) {
	if f == nil || f.manager == nil {
		return ast.ErrASTHandle
	}
	if f.closed {
		return nil
	}
	for len(f.refs) != 0 {
		i := len(f.refs) - 1
		result = errors.Join(result, f.manager.DelRef(&f.refs[i]))
		f.refs = f.refs[:i]
	}
	for len(f.held) != 0 {
		i := len(f.held) - 1
		result = errors.Join(result, f.manager.DelRef(&f.held[i]))
		f.held = f.held[:i]
	}
	if f.model != nil {
		consumed, err := f.model.DecRefConsumed()
		result = errors.Join(result, err)
		if !consumed {
			return errors.Join(result, tactic.ErrModelConverterOwner)
		}
		f.model = nil
	}
	if result == nil {
		f.decoded = opt.DecodedFormulaSet{}
		f.closed = true
	}
	return result
}

func (c *Context) simplifyOptimizationFormulas(set *opt.ObjectiveFormulaSet) (result *optimizationSimplifiedFormulas, resultErr error) {
	if c == nil || c.manager == nil || set == nil {
		return nil, ast.ErrASTHandle
	}
	result = &optimizationSimplifiedFormulas{manager: c.manager}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	// simplify_fmls returns (its fmls.reset() and its result goals and
	// input goal gone) before from_fmls decodes the objectives.
	formulas, err := c.simplifyOptimizationGoal(set, result)
	if err != nil {
		return nil, err
	}
	if result.inconsistent {
		return result, nil
	}
	if err = c.decodeOptimizationFormulas(set, formulas, result); err != nil {
		return nil, err
	}
	return result, nil
}

// simplifyOptimizationGoal is simplify_fmls: it runs m_simplify on a goal
// of set's formulas and returns the result goal's formulas, which
// result.refs holds (normalize's fmls vector).
func (c *Context) simplifyOptimizationGoal(set *opt.ObjectiveFormulaSet, result *optimizationSimplifiedFormulas) (formulas []ast.ASTHandle, resultErr error) {
	formulas, err := set.Handles()
	if err != nil {
		return nil, err
	}
	goal, err := tactic.NewGoal(c.manager, false, true, false)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, goal.Close()) }()
	for _, formula := range formulas {
		if err = goal.Assert(formula); err != nil {
			return nil, err
		}
	}
	var owned []tactic.Tactic
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, owned[i].DecRef())
		}
	}()
	keep := func(value tactic.Tactic, cause error) (tactic.Tactic, error) {
		if cause != nil {
			return nil, cause
		}
		if cause = value.IncRef(); cause != nil {
			return nil, cause
		}
		owned = append(owned, value)
		return value, nil
	}
	var stages []tactic.Tactic
	appendStage := func(value tactic.Tactic, cause error) error {
		value, cause = keep(value, cause)
		if cause == nil {
			stages = append(stages, value)
		}
		return cause
	}
	m, params, globals := c.manager, c.config.Params, c.config.Globals
	// opt::context's m_params: its constructor sets model, unsat_core and
	// elim_to_real, and updt_params appends the user's settings. Only
	// tac0's first simplify gets them; the other stages are made with
	// default parameters.
	optimizeParams, err := params.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, params.Del(&optimizeParams)) }()
	for _, name := range []string{"model", "unsat_core", "elim_to_real"} {
		key := name
		if err = params.SetBoolName(&optimizeParams, &key, true); err != nil {
			return nil, err
		}
	}
	if err = params.Copy(&optimizeParams, &c.params); err != nil {
		return nil, err
	}
	if err = appendStage(tacticcore.MakeSimplifyTactic(m, params, globals, &optimizeParams)); err == nil {
		// mk_propagate_values_tactic is not wrapped in clean(): its imp
		// (goal_shared_occs included) persists until the tactic dies.
		var propagate *tacticcore.PropagateValuesTactic
		propagate, err = tacticcore.NewPropagateValuesTactic(m, params, globals, nil)
		if err == nil {
			err = appendStage(propagate, nil)
		}
	}
	if err == nil {
		err = appendStage(tacticcore.MakeSolveEquationsTactic(m, params, globals, nil, c.config.ModelEvaluator))
	}
	if err == nil {
		err = appendStage(tacticcore.MakeSimplifyTactic(m, params, globals, nil))
	}
	optParams, paramsErr := opt.NewParams(params, globals, &c.params)
	err = errors.Join(err, paramsErr)
	if err == nil {
		var enabled bool
		enabled, err = optParams.Elim01()
		if err == nil && enabled {
			var datatype *bv.DT2BVTactic
			datatype, err = bv.NewDT2BVTactic(m, params, nil, c.config.ModelEvaluator)
			if err == nil {
				var clean tactic.Tactic
				clean, err = tactic.NewCleanTactic(datatype)
				if err == nil {
					err = appendStage(clean, nil)
				}
				if err != nil && clean == nil {
					err = errors.Join(err, datatype.Close())
				}
			}
			if err == nil {
				err = appendStage(arith.MakeElim01Tactic(m, params, globals, nil, c.config.ModelEvaluator))
			}
			var compileEquality bool
			if err == nil {
				compileEquality, err = optParams.PBCompileEquality()
			}
			if err == nil {
				var local uparams.ParamsRef
				local, err = params.NewValue()
				if err == nil {
					key := "compile_equality"
					err = params.SetBoolName(&local, &key, compileEquality)
					if err == nil {
						err = appendStage(arith.MakeLIA2CardTactic(m, params, globals, &local))
					}
					err = errors.Join(err, params.Del(&local))
				}
			}
			if err == nil {
				err = appendStage(arith.MakeEq2BVTactic(m, params, globals))
			}
			if err == nil {
				err = appendStage(tacticcore.MakeSimplifyTactic(m, params, globals, nil))
			}
		}
	}
	if optParams != nil {
		err = errors.Join(err, optParams.Close())
	}
	if err != nil {
		return nil, err
	}
	sequence, err := tactic.NewAndThenTactic(stages...)
	if err != nil {
		return nil, err
	}
	if err = sequence.IncRef(); err != nil {
		return nil, err
	}
	// set_simplify: opt::context::m_simplify keeps the chain (and its
	// tactics' state) until the next simplify_fmls replaces it; the run
	// then starts with mc = nullptr, releasing m_model_converter.
	previous := c.optimizationSimplify
	c.optimizationSimplify = sequence
	if previous != nil {
		if err = previous.DecRef(); err != nil {
			return nil, err
		}
	}
	if err = c.releaseOptimizationModelConverter(); err != nil {
		return nil, err
	}
	var applied tactic.TacticResult
	defer func() { resultErr = errors.Join(resultErr, applied.Close()) }()
	if err = sequence.Apply(goal, &applied); err != nil {
		return nil, err
	}
	if applied.ModelConverter != nil {
		if err = applied.ModelConverter.IncRef(); err != nil {
			return nil, err
		}
		c.optimizationModelConverter = applied.ModelConverter
	}
	if len(applied.Goals) != 1 {
		return nil, ast.ErrASTDomain
	}
	output := applied.Goals[0]
	inconsistent, err := output.Inconsistent()
	if err != nil {
		return nil, err
	}
	if inconsistent {
		result.inconsistent = true
		return nil, nil
	}
	count, err := output.Size()
	if err != nil {
		return nil, err
	}
	if applied.ModelConverter != nil {
		if err = applied.ModelConverter.IncRef(); err != nil {
			return nil, err
		}
		result.model = applied.ModelConverter
	}
	// fmls.reset(): to_fmls's formulas, the original objective-function
	// applications included, go before the simplified ones are copied.
	if err = set.ReleaseFormulas(); err != nil {
		return nil, err
	}
	formulas = make([]ast.ASTHandle, count)
	for i := range formulas {
		formulas[i], err = output.Form(uint32(i))
		if err != nil {
			return nil, err
		}
		ref, e := m.NewRef(formulas[i])
		if e != nil {
			return nil, e
		}
		result.refs = append(result.refs, ref)
	}
	return formulas, nil
}

func (c *Context) decodeOptimizationFormulas(set *opt.ObjectiveFormulaSet, formulas []ast.ASTHandle, result *optimizationSimplifiedFormulas) (err error) {
	m := c.manager
	result.decoded, err = set.DecodeHandles(formulas)
	if err != nil {
		return err
	}
	// from_fmls keeps the hard constraints and each objective's terms in
	// the context; normalize's fmls vector (refs) is released by
	// releaseFormulas once from_fmls is done.
	hold := func(h ast.ASTHandle) error {
		if h.IsNull() {
			return nil
		}
		ref, e := m.NewRef(h)
		if e != nil {
			return e
		}
		result.held = append(result.held, ref)
		return nil
	}
	for _, h := range result.decoded.Hard {
		if err = hold(h); err != nil {
			return err
		}
	}
	for _, objective := range result.decoded.Objectives {
		for _, h := range append([]ast.ASTHandle{objective.Term, objective.OriginalTerm}, objective.SoftTerms...) {
			if err = hold(h); err != nil {
				return err
			}
		}
	}
	return nil
}

// releaseFormulas is the end of normalize: its fmls vector (the simplified
// formulas, objective-function applications included) goes front to back.
func (f *optimizationSimplifiedFormulas) releaseFormulas() (result error) {
	for i := range f.refs {
		result = errors.Join(result, f.manager.DelRef(&f.refs[i]))
	}
	f.refs = nil
	return result
}

// releaseOptimizationModelConverter drops opt::context::m_model_converter.
func (c *Context) releaseOptimizationModelConverter() error {
	if c.optimizationModelConverter == nil {
		return nil
	}
	mc := c.optimizationModelConverter
	c.optimizationModelConverter = nil
	return mc.DecRef()
}

// releaseOptimizationSimplifier drops m_simplify and m_model_converter
// with the opt::context.
func (c *Context) releaseOptimizationSimplifier() error {
	var err error
	if c.optimizationSimplify != nil {
		t := c.optimizationSimplify
		c.optimizationSimplify = nil
		err = t.DecRef()
	}
	return errors.Join(err, c.releaseOptimizationModelConverter())
}
