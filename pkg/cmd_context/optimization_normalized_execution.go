package cmd_context

// Source: src/opt/opt_context.cpp is_maxsat/from_fmls before objective execution.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (c *Context) runNormalizedOptimization(assumptions []ast.ASTHandle, continuation bool, priority string, paretoContinuation, boxContinuation bool) (optimized *opt.ArithmeticResult, resultErr error) {
	original := c.objectives
	source := opt.DecodedFormulaSet{Objectives: make([]opt.DecodedFormulaObjective, len(original))}
	for i, objective := range original {
		kind := opt.FormulaMinimize
		if objective.maximize {
			kind = opt.FormulaMaximize
		}
		source.Objectives[i] = opt.DecodedFormulaObjective{Kind: kind, Term: objective.expression, OriginalTerm: objective.expression}
		if objective.soft {
			source.Objectives[i].Kind = opt.FormulaMaxSAT
			for _, term := range objective.softTerms {
				source.Objectives[i].SoftTerms = append(source.Objectives[i].SoftTerms, term.expression)
				source.Objectives[i].Weights = append(source.Objectives[i].Weights, *new(big.Rat).Set(&term.weight))
			}
		}
	}
	decomposer, err := opt.NewObjectiveDecomposer(c.manager, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		c.objectives = original
		resultErr = errors.Join(resultErr, decomposer.Close())
		if resultErr != nil && optimized != nil {
			resultErr = errors.Join(resultErr, optimized.Close())
			optimized = nil
		}
	}()
	decomposed, _, err := decomposer.Decompose(source)
	if err != nil {
		return nil, err
	}
	normalized := append([]optimizationObjective(nil), original...)
	for i, d := range decomposed {
		if original[i].soft || !d.Converted {
			continue
		}
		normalized[i].soft, normalized[i].bitVector = true, false
		normalized[i].softTerms = make([]optimizationSoftTerm, len(d.SoftTerms))
		for j, term := range d.SoftTerms {
			normalized[i].softTerms[j].expression = term
			normalized[i].softTerms[j].weight.Set(&d.Weights[j])
		}
	}
	c.objectives = normalized
	if paretoContinuation {
		optimized, err = c.checkOptimizePareto(assumptions, true)
	} else if boxContinuation {
		optimized, err = c.checkOptimizeBox(assumptions, true)
	} else if priority == "pareto" {
		if err = c.clearOptimizationBoxModels(); err != nil {
			return nil, err
		}
		c.optimizationBoxActive = false
		optimized, err = c.checkOptimizePareto(assumptions, continuation)
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
	if optimized.Status != lbool.True {
		if err = restoreNormalizedObjectiveBounds(optimized, original, decomposed); err != nil {
			return optimized, err
		}
	}
	if optimized.Status == lbool.True {
		for i, d := range decomposed {
			if original[i].soft || original[i].bitVector || !d.Converted {
				continue
			}
			value, e := d.RestoreValue(&optimized.Values[i].SoftCost)
			if e != nil {
				return optimized, e
			}
			finite := simplex.RationalInfinitesimal(value)
			optimized.Values[i] = opt.ObjectiveValue{
				Arithmetic:           smtcore.SMTArithmeticObjectiveValue{Finite: finite},
				ArithmeticModel:      smtcore.SMTArithmeticObjectiveValue{Finite: finite.Clone()},
				ArithmeticModelValid: true,
			}
		}
	}
	return optimized, nil
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
