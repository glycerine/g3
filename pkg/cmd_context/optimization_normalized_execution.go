package cmd_context

// Source: src/opt/opt_context.cpp is_maxsat/from_fmls before objective execution.

import (
	"errors"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/numeral"

	"github.com/glycerine/g3/pkg/ast"
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
	// source owns its objectives' rationals (simplify's decoded ones and
	// the originals) until the run ends.
	defer func() { resultErr = errors.Join(resultErr, source.ReleaseRationals(c.optQ())) }()
	inconsistent := false
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
			inconsistent = true
		} else {
			// from_fmls installs the new objectives and hard constraints;
			// the context keeps them until the next import_scoped_state.
			c.optimizationTerms, simplified.held = simplified.held, nil
			source = simplified.decoded
			// source now owns the decoded objectives' rationals.
			simplified.decoded.Objectives = nil
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
					byIndex[i] = c.originalDecodedObjective(&original[i])
					byIndex[i].HardPosition = len(source.Hard)
				}
			}
			source.Objectives = byIndex
		}
	}
	if !useSimplifier {
		source.Objectives = make([]opt.DecodedFormulaObjective, len(original))
		for i := range original {
			source.Objectives[i] = c.originalDecodedObjective(&original[i])
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
		c.optimizationUnconverted = false
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
		decomposer.SetEach(func(index int, objective opt.DecomposedObjective) error {
			position := len(source.Hard)
			if index < len(source.Objectives) {
				position = source.Objectives[index].HardPosition
			}
			if e := purifier.AdvanceHard(position); e != nil {
				return e
			}
			_, e := purifier.PurifyObjective(objective)
			return e
		})
	}
	// An inconsistent goal reaches from_fmls as the single formula false,
	// so no objective is converted to maxsat (AUDIT.md FUZZ_A-F1).
	decomposer.SetUnconverted(inconsistent)
	c.optimizationUnconverted = inconsistent
	decomposed, _, err := decomposer.Decompose(source)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, opt.ReleaseDecomposed(c.optQ(), decomposed)) }()
	var purified *opt.PurifiedObjectiveSet
	if simplified != nil {
		if err = purifier.AdvanceHard(-1); err != nil {
			return nil, err
		}
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
	// The normalized objectives are opt::context's m_objectives after
	// from_fmls: their own rationals, released when the original objectives
	// return.
	normalized, err := c.cloneObjectives(original)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.releaseObjectives(normalized)) }()
	q := c.optQ()
	// setSoftTerms replaces an objective's soft terms with terms/weights.
	setSoftTerms := func(o *optimizationObjective, terms []ast.ASTHandle, weights []numeral.Rational) error {
		err := c.releaseSoftTerms(o.softTerms)
		o.softTerms = make([]optimizationSoftTerm, len(terms))
		for j, term := range terms {
			o.softTerms[j].expression = term
			o.softTerms[j].weight = numeral.NewRational()
			err = errors.Join(err, q.Copy(&o.softTerms[j].weight, &weights[j]))
		}
		return err
	}
	var bridgeErr error
	for i, d := range decomposed {
		if i < len(source.Objectives) {
			objective := source.Objectives[i]
			if objective.Kind == opt.FormulaMaxSAT {
				bridgeErr = errors.Join(bridgeErr, setSoftTerms(&normalized[i], objective.SoftTerms, objective.Weights),
					q.Copy(&normalized[i].softOffset, &objective.Offset))
			} else {
				normalized[i].expression = objective.Term
				normalized[i].maximize = objective.Kind == opt.FormulaMaximize
			}
		}
		if !original[i].soft && d.Converted {
			normalized[i].verifyConverted, normalized[i].verifyNegate = true, d.Negate
			bridgeErr = errors.Join(bridgeErr, q.Copy(&normalized[i].verifyOffset, &d.Offset))
			normalized[i].soft, normalized[i].bitVector = true, false
			bridgeErr = errors.Join(bridgeErr, setSoftTerms(&normalized[i], d.SoftTerms, d.Weights))
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
	if bridgeErr != nil {
		return nil, bridgeErr
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
	if inconsistent {
		// simplify_fmls leaves the inconsistent goal's single formula
		// false, and from_fmls makes it all of m_hard_constraints: the
		// solver checks false, not the original assertions.
		falseFormula, e := c.manager.MkFalse()
		if e != nil {
			return nil, e
		}
		c.optimizationHardOverride = []ast.ASTHandle{falseFormula}
		c.optimizationHardOverrideActive = true
	}
	// optimize(): a single objective runs execute(m_objectives[0], true,
	// false) whatever the priority; only several objectives reach
	// execute_pareto or execute_box. (The m_pareto1 toggle set there is
	// cleared by the next optimize()'s clear_state before it is read.)
	if !paretoContinuation && !boxContinuation {
		// An exception from here on comes after normalize() and
		// internalize(): display_assignment reads the bounds optsmt::add
		// and maxsmt::add left, through the normalized m_adjust_value.
		values := opt.NewObjectiveValues(c.optQ(), len(c.objectives))
		c.optimizationFailureDisplay = values
		if err = c.initialDisplayBounds(values); err != nil {
			return nil, err
		}
	}
	if boxContinuation {
		optimized, err = c.checkOptimizeBox(assumptions, true)
	} else if priority == "pareto" && len(c.objectives) > 1 {
		if err = c.retireOptimizationBoxModels(); err != nil {
			return nil, err
		}
		c.optimizationBoxActive = false
		optimized, err = c.checkOptimizePareto(assumptions, continuation)
		if c.optimizationParetoPurification != nil {
			// A later execute_pareto reports values through this
			// normalization; keep its value transforms (also when the
			// engine threw: m_pareto and m_objectives survive it).
			if e := opt.ReleaseDecomposed(c.optQ(), c.optimizationParetoPurification.decomposed); e != nil {
				err = errors.Join(err, e)
			}
			c.optimizationParetoPurification.decomposed = c.paretoValueTransforms(decomposed)
		}
	} else if priority == "box" && len(c.objectives) > 1 {
		optimized, err = c.checkOptimizeBox(assumptions, continuation)
	} else {
		if err = c.retireOptimizationBoxModels(); err != nil {
			return nil, err
		}
		c.optimizationBoxActive = false
		optimized, err = c.checkOptimizeLex(assumptions)
	}

	if err != nil {
		if c.optimizationFailureNormalized && c.optimizationFailureDisplay != nil && len(c.optimizationFailureDisplay) == len(original) {
			// Converted objectives display m_adjust_value of their maxsmt
			// bounds.
			if e := c.restoreNormalizedObjectiveBounds(&opt.ArithmeticResult{Q: c.optQ(), Values: c.optimizationFailureDisplay}, original, decomposed); e != nil {
				err = errors.Join(err, e)
			}
			c.optimizationFailureNormalized = false
		}
		return optimized, err
	}
	if err = c.restoreNormalizedValues(optimized, original, decomposed, priority, boxContinuation); err != nil {
		return optimized, err
	}
	return optimized, nil
}

// paretoValueTransforms keeps only the value-restoring fields of a
// decomposition (its term handles die with the decomposer).
// The result owns its offsets (opt.ReleaseDecomposed).
func (c *Context) paretoValueTransforms(decomposed []opt.DecomposedObjective) []opt.DecomposedObjective {
	q := c.optQ()
	out := make([]opt.DecomposedObjective, len(decomposed))
	for i := range decomposed {
		out[i].SourceKind = decomposed[i].SourceKind
		out[i].Offset = numeral.NewRational()
		_ = q.Copy(&out[i].Offset, &decomposed[i].Offset)
		out[i].Negate = decomposed[i].Negate
		out[i].Converted = decomposed[i].Converted
	}
	return out
}

// restoreNormalizedValues maps an execution's values back through the
// objective conversion.
func (c *Context) restoreNormalizedValues(optimized *opt.ArithmeticResult, original []optimizationObjective, decomposed []opt.DecomposedObjective, priority string, boxContinuation bool) error {
	if optimized.Status != lbool.True {
		if err := c.restoreNormalizedObjectiveBounds(optimized, original, decomposed); err != nil {
			return err
		}
	}
	if optimized.Status == lbool.True {
		for i, d := range decomposed {
			if original[i].soft || original[i].bitVector || !d.Converted {
				continue
			}
			if optimized.Values[i].SoftInterval {
				// This maxsmt never ran (execute_lex returned on an
				// unbounded objective), so display_assignment shows
				// m_adjust_value of its m_lower and m_upper as an interval
				// (AUDIT.md FUZZ_A-F3).
				lower, e := c.optRestoreInfinitesimal(&d, &optimized.Values[i].SoftLower)
				if e != nil {
					return e
				}
				upper, e := c.optRestoreInfinitesimal(&d, &optimized.Values[i].SoftUpper)
				if e != nil {
					return e
				}
				// writeObjectiveValue prints (interval modelValue value) for
				// maximize and (interval value modelValue) for minimize.
				shown, model := upper, lower
				if !original[i].maximize {
					shown, model = lower, upper
				}
				if e = c.optReplaceValue(&optimized.Values[i]); e != nil {
					return e
				}
				optimized.Values[i].Arithmetic = smtcore.SMTArithmeticObjectiveValue{Finite: shown}
				optimized.Values[i].ArithmeticModel = smtcore.SMTArithmeticObjectiveValue{Finite: model}
				optimized.Values[i].ArithmeticModelValid = true
				optimized.Values[i].ArithmeticInterval = true
				continue
			}
			finite, e := c.optRestoreInfinitesimal(&d, &optimized.Values[i].SoftCost)
			if e != nil {
				return e
			}
			if e = c.optReplaceValue(&optimized.Values[i]); e != nil {
				return e
			}
			optimized.Values[i].Arithmetic = smtcore.SMTArithmeticObjectiveValue{Finite: finite}
			optimized.Values[i].ArithmeticModel = smtcore.SMTArithmeticObjectiveValue{Finite: finite.Clone()}
			optimized.Values[i].ArithmeticModelValid = true
		}
		// execute_box's later calls report the values the first call
		// computed, in source terms.
		if priority == "box" && !boxContinuation && c.optimizationBoxModels != nil {
			c.optimizationBoxValues = c.cloneOptimizationBoxValues(optimized.Values)
		}
	}
	return nil
}

// MaxSMT penalties have the opposite order when normalization negates the
// objective. Restore both endpoints before exposing an interrupted result.
func (c *Context) restoreNormalizedObjectiveBounds(result *opt.ArithmeticResult, original []optimizationObjective, decomposed []opt.DecomposedObjective) error {
	for i := range original {
		if original[i].soft || i >= len(decomposed) || i >= len(result.Values) || !decomposed[i].Converted {
			continue
		}
		value := &result.Values[i]
		if !value.SoftBoundsValid {
			continue
		}
		if value.DisplayValid {
			// get_lower_as_num/get_upper_as_num of a converted objective
			// are m_adjust_value of the maxsmt bounds, unswapped
			// (AUDIT.md FUZZ_A-F5).
			displayLower, err := c.optRestoreInfinitesimal(&decomposed[i], &value.SoftLower)
			if err != nil {
				return err
			}
			displayUpper, err := c.optRestoreInfinitesimal(&decomposed[i], &value.SoftUpper)
			if err != nil {
				return err
			}
			value.DisplayLower = displayLower
			value.DisplayUpper = displayUpper
		}
		lowerPenalty, upperPenalty := &value.SoftLower, &value.SoftUpper
		if decomposed[i].Negate {
			lowerPenalty, upperPenalty = upperPenalty, lowerPenalty
		}
		q := c.optQ()
		// lowerPenalty/upperPenalty alias the value's own soft bounds, which
		// the restored source bounds do not overwrite.
		if err := decomposed[i].RestoreValue(q, lowerPenalty, &value.SourceLower); err != nil {
			return err
		}
		if err := decomposed[i].RestoreValue(q, upperPenalty, &value.SourceUpper); err != nil {
			return err
		}
		value.SourceBoundsValid = true
	}
	return nil
}

// originalDecodedObjective is an objective as m_objectives holds it before
// from_fmls.
// The result owns its rationals (DecodedFormulaObjective.ReleaseRationals).
func (c *Context) originalDecodedObjective(objective *optimizationObjective) opt.DecodedFormulaObjective {
	kind := opt.FormulaMinimize
	if objective.maximize {
		kind = opt.FormulaMaximize
	}
	result := opt.DecodedFormulaObjective{Kind: kind, Term: objective.expression, OriginalTerm: objective.expression, Offset: numeral.NewRational()}
	if objective.soft {
		result.Kind = opt.FormulaMaxSAT
		for _, term := range objective.softTerms {
			result.SoftTerms = append(result.SoftTerms, term.expression)
			weight, _ := c.optClone(&term.weight)
			result.Weights = append(result.Weights, weight)
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
	return purifier, purifier.BeginPending(hard)
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
