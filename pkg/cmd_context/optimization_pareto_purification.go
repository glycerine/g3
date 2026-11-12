package cmd_context

// Source: src/opt/opt_context.cpp:950-997,1062-1137,336-343.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/tactic"
)

// The same definitions must survive all calls in one frontier enumeration.
type paretoPurification struct {
	purifier   *opt.ObjectivePurifier
	objectives []optimizationObjective
	hard       []ast.ASTHandle
	// converter is normalize's model converter (m_model_converter), held
	// for every point of the frontier.
	converter tactic.ModelConverter
	// decomposed holds the normalization's value transforms (offset,
	// negation) for the continued execute_pareto results.
	decomposed []opt.DecomposedObjective
}

func (c *Context) prepareParetoPurification() (resultErr error) {
	purifier, err := opt.NewObjectivePurifier(c.manager)
	if err != nil {
		return err
	}
	state := &paretoPurification{purifier: purifier}
	c.optimizationParetoPurification = state
	if c.optimizationPreprocessor != nil {
		if err := c.optimizationPreprocessor.IncRef(); err != nil {
			return err
		}
		state.converter = c.optimizationPreprocessor
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, c.closeParetoBackend())
		}
	}()
	objectives := make([]opt.DecomposedObjective, len(c.objectives))
	for i, objective := range c.objectives {
		kind := opt.FormulaMinimize
		if objective.maximize {
			kind = opt.FormulaMaximize
		}
		objectives[i] = opt.DecomposedObjective{SourceKind: kind, Term: objective.expression, OriginalTerm: objective.expression}
		if objective.soft {
			objectives[i].SourceKind = opt.FormulaMaxSAT
			objectives[i].Term = ast.ASTHandle{}
			for _, term := range objective.softTerms {
				objectives[i].SoftTerms = append(objectives[i].SoftTerms, term.expression)
				objectives[i].Weights = append(objectives[i].Weights, *new(big.Rat).Set(&term.weight))
			}
		}
	}
	// gia_pareto runs on the normalized context: from_fmls' hard
	// constraints replace the asserted formulas.
	var hard []ast.ASTHandle
	if c.optimizationHardOverrideActive {
		hard = append(hard, c.optimizationHardOverride...)
	} else {
		hard = make([]ast.ASTHandle, len(c.assertions))
		for i := range c.assertions {
			hard[i] = c.assertions[i].expression
		}
	}
	set, err := purifier.Purify(hard, objectives)
	if err != nil {
		return err
	}
	state.hard = set.Hard
	state.objectives = append([]optimizationObjective(nil), c.objectives...)
	for i, objective := range set.Objectives {
		if !state.objectives[i].soft {
			state.objectives[i].expression = objective.Term
			continue
		}
		state.objectives[i].softTerms = make([]optimizationSoftTerm, len(objective.SoftTerms))
		for j, term := range objective.SoftTerms {
			state.objectives[i].softTerms[j].expression = term
			state.objectives[i].softTerms[j].weight.Set(&objective.Weights[j])
		}
	}
	return nil
}

// Keep the raw solver model for comparisons; only the published copy is fixed.
// fix_model applies m_model_converter = concat(simplify mc, purify filter):
// the purification filter first, then the normalize converter.
func (c *Context) filterParetoModel(raw *model.ModelCore) (_ *model.ModelCore, resultErr error) {
	state := c.optimizationParetoPurification
	filtered, err := state.purifier.FilterModel(raw)
	if err != nil {
		return filtered, err
	}
	if state.converter == nil && c.optimizationParetoOptimizer == nil {
		return filtered, nil
	}
	var reference tactic.ModelRef
	if err = reference.Set(filtered); err != nil {
		return nil, errors.Join(err, filtered.DecRef())
	}
	defer func() { resultErr = errors.Join(resultErr, reference.Close()) }()
	if err = filtered.DecRef(); err != nil {
		return nil, err
	}
	if state.converter != nil {
		if err = state.converter.ApplyModel(&reference, 0); err != nil {
			return nil, err
		}
	}
	// fix_model ends with m_fm(mdl, 0): mk_ge's bound constants.
	if err = c.optimizationParetoOptimizer.ApplyObjectiveFilter(&reference); err != nil {
		return nil, err
	}
	output, err := reference.Get()
	if err != nil || output == nil {
		if err == nil {
			err = ast.ErrASTHandle
		}
		return nil, err
	}
	if err = output.IncRef(); err != nil {
		return nil, err
	}
	return output, nil
}
