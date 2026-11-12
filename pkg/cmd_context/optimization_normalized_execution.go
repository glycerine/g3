package cmd_context

// Source: src/opt/opt_context.cpp is_maxsat/from_fmls before objective execution.

import (
	"errors"
	"github.com/glycerine/g3/pkg/tactic"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (c *Context) runNormalizedOptimization(assumptions []ast.ASTHandle, continuation bool, priority string, paretoContinuation, boxContinuation bool) (optimized *opt.ArithmeticResult, resultErr error) {
	original := c.objectives
	originalHardOverride := c.optimizationHardOverride
	originalHardOverrideActive := c.optimizationHardOverrideActive
	originalPreprocessor := c.optimizationPreprocessor
	// Lexicographic execution owns one ArithmeticSolver and can carry the
	// tactic converter through its model publication path. Pareto and box
	// maintain separate long-lived model paths; weighted soft objectives also
	// keep their source MaxSMT search and resource accounting intact.
	settings, err := c.optimizationSettings()
	if err != nil {
		return nil, err
	}
	// optimize() with an m_pareto engine returns execute_pareto() at once:
	// no clear_state, init_solver, normalize or objective conversion.
	if paretoContinuation {
		var decomposed []opt.DecomposedObjective
		if state := c.optimizationParetoPurification; state != nil {
			decomposed = state.decomposed
		}
		if optimized, err = c.checkOptimizePareto(assumptions, true); err != nil {
			return optimized, err
		}
		if err = c.restoreNormalizedValues(optimized, original, decomposed, priority, false); err != nil {
			return nil, errors.Join(err, optimized.Close())
		}
		return optimized, nil
	}
	// optimize() always runs normalize() unless it resumes Pareto or box
	// execution.
	_ = settings
	// Pareto skips normalize: z3 4.7's gia_pareto does not terminate once
	// normalize turns 0-1 objectives into MaxSAT sums (z3 times out on
	// test vectors the frontier enumeration here completes).
	useSimplifier := !paretoContinuation && !boxContinuation
	// optimize(): init_solver precedes normalize(), Pareto included.
	// Recorded deviation (TODO item 30): z3 4.7's gia_pareto does not
	// terminate once normalize turns a 0-1 maximize/minimize objective into
	// a MaxSAT sum, so that one case keeps the un-normalized objectives.
	if !paretoContinuation && !boxContinuation {
		// clear_state: m_model.reset() before init_solver.
		if err = c.releaseParetoInitialModel(); err != nil {
			return nil, err
		}
		if c.preparedOptimizer, err = c.createOptimizer(); err != nil {
			return nil, err
		}
		// init_solver: m_opt_solver = alloc(...) releases the previous one
		// after the new kernel exists.
		if err = c.releaseRetainedOptimizer(); err != nil {
			return nil, err
		}
		if err = c.closeParetoBackend(); err != nil {
			return nil, err
		}
		defer func() {
			if c.preparedOptimizer != nil {
				resultErr = errors.Join(resultErr, c.preparedOptimizer.Close())
				c.preparedOptimizer = nil
			}
		}()
	}
	// import_scoped_state (after init_solver) resets the previous run's
	// optsmt objectives, objectives and hard constraints. A Pareto or box
	// continuation returns from optimize() before it.
	if !paretoContinuation && !boxContinuation {
		if err = c.releaseOptimizationTerms(); err != nil {
			return nil, err
		}
	}
	var formulaSet *opt.ObjectiveFormulaSet
	var simplified *optimizationSimplifiedFormulas
	var source opt.DecodedFormulaSet
	if useSimplifier {
		formulaSet, err = c.EncodeOptimizationFormulas()
		if err != nil {
			return nil, err
		}
		c.optimizationObjectiveRefs = append(c.optimizationObjectiveRefs, formulaSet.DetachDeclarations()...)
		simplified, err = c.simplifyOptimizationFormulas(formulaSet)
		if err != nil {
			return nil, errors.Join(err, formulaSet.Close())
		}
		// simplify_fmls ends with fmls.reset(): to_fmls's formulas, the
		// original objective-function applications included, are
		// released before from_fmls.
		err = formulaSet.Close()
		formulaSet = nil
		if err != nil {
			return nil, errors.Join(err, simplified.Close())
		}
		if simplified.inconsistent {
			if err = simplified.Close(); err != nil {
				return nil, errors.Join(err, formulaSet.Close())
			}
			simplified = nil
			useSimplifier = false
		} else {
			// from_fmls installs the new objectives and hard constraints;
			// the context keeps them until the next import_scoped_state.
			c.optimizationTerms, simplified.held = simplified.held, nil
			source = simplified.decoded
			// from_fmls updates the objectives it finds; one whose
			// function application was simplified away keeps its terms.
			byIndex := make([]opt.DecodedFormulaObjective, len(original))
			found := make([]bool, len(original))
			for _, objective := range source.Objectives {
				if objective.Index < 0 || objective.Index >= len(original) || found[objective.Index] {
					return nil, errors.Join(ast.ErrASTDomain, simplified.Close())
				}
				byIndex[objective.Index], found[objective.Index] = objective, true
			}
			for i := range byIndex {
				if !found[i] {
					byIndex[i] = originalDecodedObjective(&original[i])
				}
			}
			source.Objectives = byIndex
		}
	}
	if !useSimplifier {
		source.Objectives = make([]opt.DecodedFormulaObjective, len(original))
		for i := range original {
			source.Objectives[i] = originalDecodedObjective(&original[i])
		}
	}
	decomposer, err := opt.NewObjectiveDecomposer(c.manager, nil)
	if err != nil {
		if simplified != nil {
			err = errors.Join(err, simplified.Close())
		}
		if formulaSet != nil {
			err = errors.Join(err, formulaSet.Close())
		}
		return nil, err
	}
	defer func() {
		c.objectives = original
		c.optimizationHardOverride = originalHardOverride
		c.optimizationHardOverrideActive = originalHardOverrideActive
		c.optimizationPreprocessor = originalPreprocessor
		if decomposer != nil {
			resultErr = errors.Join(resultErr, decomposer.Close())
		}
		if simplified != nil {
			resultErr = errors.Join(resultErr, simplified.Close())
		}
		if formulaSet != nil {
			resultErr = errors.Join(resultErr, formulaSet.Close())
		}
		if resultErr != nil && optimized != nil {
			resultErr = errors.Join(resultErr, optimized.Close())
			optimized = nil
		}
	}()
	// from_fmls handles each objective formula in turn: is_maxsat, then
	// mk_atomic on its soft terms or purify(tr) on its arithmetic term,
	// pushing (= q t) onto m_hard_constraints; fm joins m_model_converter.
	var purifier *opt.ObjectivePurifier
	if simplified != nil {
		if purifier, err = c.beginNormalizedPurification(source.Hard); err != nil {
			return nil, err
		}
		decomposer.SetEach(func(_ int, objective opt.DecomposedObjective) error {
			_, e := purifier.PurifyObjective(objective)
			return e
		})
	}
	decomposed, _, err := decomposer.Decompose(source)
	if err != nil {
		return nil, err
	}
	var purified *opt.PurifiedObjectiveSet
	if simplified != nil {
		if purified, err = c.finishNormalizedPurification(purifier); err != nil {
			return nil, err
		}
		// The purifier now owns m_hard_constraints and the objectives'
		// terms. is_maxsat's term vectors and the decoded objective terms
		// die with their from_fmls iteration; the normalize fmls still
		// hold the objective applications until releaseFormulas.
		for i := range c.optimizationTerms {
			err = errors.Join(err, c.manager.DelRef(&c.optimizationTerms[i]))
		}
		c.optimizationTerms = nil
		err = errors.Join(err, decomposer.Close())
		decomposer = nil
		if err != nil {
			return nil, err
		}
	}
	// End of normalize(): its fmls vector, the objective-function
	// applications included, is released once from_fmls is done.
	if simplified != nil {
		if err = simplified.releaseFormulas(); err != nil {
			return nil, err
		}
	}
	if formulaSet != nil {
		err = formulaSet.Close()
		formulaSet = nil
		if err != nil {
			return nil, err
		}
	}
	normalized := append([]optimizationObjective(nil), original...)
	for i, d := range decomposed {
		if i < len(source.Objectives) {
			objective := source.Objectives[i]
			if objective.Kind == opt.FormulaMaxSAT {
				normalized[i].softTerms = make([]optimizationSoftTerm, len(objective.SoftTerms))
				for j, term := range objective.SoftTerms {
					normalized[i].softTerms[j].expression = term
					normalized[i].softTerms[j].weight.Set(&objective.Weights[j])
				}
				normalized[i].softOffset.Set(&objective.Offset)
			} else {
				normalized[i].expression = objective.Term
				normalized[i].maximize = objective.Kind == opt.FormulaMaximize
			}
		}
		if !original[i].soft && d.Converted {
			normalized[i].verifyConverted, normalized[i].verifyNegate = true, d.Negate
			normalized[i].verifyOffset.Set(&d.Offset)
			normalized[i].soft, normalized[i].bitVector = true, false
			normalized[i].softTerms = make([]optimizationSoftTerm, len(d.SoftTerms))
			for j, term := range d.SoftTerms {
				normalized[i].softTerms[j].expression = term
				normalized[i].softTerms[j].weight.Set(&d.Weights[j])
			}
		}
		if purified == nil || i >= len(purified.Objectives) {
			continue
		}
		p := purified.Objectives[i]
		// from_fmls: obj.m_term = orig_term for a MaxSMT objective.
		if normalized[i].soft && !p.OriginalTerm.IsNull() {
			normalized[i].expression = p.OriginalTerm
		}
		if normalized[i].soft {
			if len(p.SoftTerms) != len(normalized[i].softTerms) {
				return nil, ast.ErrASTDomain
			}
			for j := range p.SoftTerms {
				normalized[i].softTerms[j].expression = p.SoftTerms[j]
			}
		} else if !p.Term.IsNull() {
			normalized[i].expression = p.Term
		}
	}
	c.objectives = normalized
	if simplified != nil {
		c.optimizationHardOverride = source.Hard
		if purified != nil {
			c.optimizationHardOverride = purified.Hard
		}
		c.optimizationHardOverrideActive = true
		c.optimizationPreprocessor = simplified.model
		if c.optimizationModelConverter != nil && c.optimizationModelConverter != simplified.model {
			c.optimizationPreprocessor = c.optimizationModelConverter
		}
	}
	if boxContinuation {
		optimized, err = c.checkOptimizeBox(assumptions, true)
	} else if priority == "pareto" {
		if err = c.clearOptimizationBoxModels(); err != nil {
			return nil, err
		}
		c.optimizationBoxActive = false
		optimized, err = c.checkOptimizePareto(assumptions, continuation)
		if err == nil && c.optimizationParetoPurification != nil {
			// A later execute_pareto reports values through this
			// normalization; keep its value transforms.
			c.optimizationParetoPurification.decomposed = paretoValueTransforms(decomposed)
		}
	} else if priority == "box" {
		optimized, err = c.checkOptimizeBox(assumptions, continuation)
	} else {
		if err = c.clearOptimizationBoxModels(); err != nil {
			return nil, err
		}
		c.optimizationBoxActive = false
		optimized, err = c.checkOptimizeLex(assumptions)
	}

	if err != nil {
		return optimized, err
	}
	if err = c.restoreNormalizedValues(optimized, original, decomposed, priority, boxContinuation); err != nil {
		return optimized, err
	}
	return optimized, nil
}

// paretoValueTransforms keeps only the value-restoring fields of a
// decomposition (its term handles die with the decomposer).
func paretoValueTransforms(decomposed []opt.DecomposedObjective) []opt.DecomposedObjective {
	out := make([]opt.DecomposedObjective, len(decomposed))
	for i := range decomposed {
		out[i].SourceKind = decomposed[i].SourceKind
		out[i].Offset.Set(&decomposed[i].Offset)
		out[i].Negate = decomposed[i].Negate
		out[i].Converted = decomposed[i].Converted
	}
	return out
}

// restoreNormalizedValues maps an execution's values back through the
// objective conversion.
func (c *Context) restoreNormalizedValues(optimized *opt.ArithmeticResult, original []optimizationObjective, decomposed []opt.DecomposedObjective, priority string, boxContinuation bool) error {
	if optimized.Status != lbool.True {
		if err := restoreNormalizedObjectiveBounds(optimized, original, decomposed); err != nil {
			return err
		}
	}
	if optimized.Status == lbool.True {
		for i, d := range decomposed {
			if original[i].soft || original[i].bitVector || !d.Converted {
				continue
			}
			value, e := d.RestoreValue(&optimized.Values[i].SoftCost)
			if e != nil {
				return e
			}
			finite := simplex.RationalInfinitesimal(value)
			optimized.Values[i] = opt.ObjectiveValue{
				Arithmetic:           smtcore.SMTArithmeticObjectiveValue{Finite: finite},
				ArithmeticModel:      smtcore.SMTArithmeticObjectiveValue{Finite: finite.Clone()},
				ArithmeticModelValid: true,
			}
		}
		// execute_box's later calls report the values the first call
		// computed, in source terms.
		if priority == "box" && !boxContinuation && c.optimizationBoxModels != nil {
			c.optimizationBoxValues = cloneOptimizationBoxValues(optimized.Values)
		}
	}
	return nil
}

// MaxSMT penalties have the opposite order when normalization negates the
// objective. Restore both endpoints before exposing an interrupted result.
func restoreNormalizedObjectiveBounds(result *opt.ArithmeticResult, original []optimizationObjective, decomposed []opt.DecomposedObjective) error {
	for i := range original {
		if original[i].soft || i >= len(decomposed) || i >= len(result.Values) || !decomposed[i].Converted {
			continue
		}
		value := &result.Values[i]
		if !value.SoftBoundsValid {
			continue
		}
		lowerPenalty, upperPenalty := &value.SoftLower, &value.SoftUpper
		if decomposed[i].Negate {
			lowerPenalty, upperPenalty = upperPenalty, lowerPenalty
		}
		lower, err := decomposed[i].RestoreValue(lowerPenalty)
		if err != nil {
			return err
		}
		upper, err := decomposed[i].RestoreValue(upperPenalty)
		if err != nil {
			return err
		}
		value.SourceLower.Set(lower)
		value.SourceUpper.Set(upper)
		value.SourceBoundsValid = true
	}
	return nil
}

// originalDecodedObjective is an objective as m_objectives holds it before
// from_fmls.
func originalDecodedObjective(objective *optimizationObjective) opt.DecodedFormulaObjective {
	kind := opt.FormulaMinimize
	if objective.maximize {
		kind = opt.FormulaMaximize
	}
	result := opt.DecodedFormulaObjective{Kind: kind, Term: objective.expression, OriginalTerm: objective.expression}
	if objective.soft {
		result.Kind = opt.FormulaMaxSAT
		for _, term := range objective.softTerms {
			result.SoftTerms = append(result.SoftTerms, term.expression)
			result.Weights = append(result.Weights, *new(big.Rat).Set(&term.weight))
		}
	}
	return result
}

// beginNormalizedPurification starts from_fmls's purification with the
// hard formulas.
func (c *Context) beginNormalizedPurification(hard []ast.ASTHandle) (*opt.ObjectivePurifier, error) {
	purifier, err := opt.NewObjectivePurifier(c.manager)
	if err != nil {
		return nil, err
	}
	c.optimizationPurifier = purifier
	return purifier, purifier.Begin(hard)
}

// finishNormalizedPurification: when purify added fresh constants,
// m_model_converter becomes concat(m_model_converter, fm).
func (c *Context) finishNormalizedPurification(purifier *opt.ObjectivePurifier) (*opt.PurifiedObjectiveSet, error) {
	set := purifier.Result()
	fm, err := purifier.DetachFilter()
	if err != nil || fm == nil {
		return &set, err
	}
	combined, err := tactic.ConcatModels(c.manager.Budget(), c.optimizationModelConverter, fm)
	if err == nil {
		err = combined.IncRef()
	}
	err = errors.Join(err, fm.DecRef())
	if err != nil {
		return nil, err
	}
	if err = c.releaseOptimizationModelConverter(); err != nil {
		return nil, errors.Join(err, combined.DecRef())
	}
	c.optimizationModelConverter = combined
	return &set, nil
}
