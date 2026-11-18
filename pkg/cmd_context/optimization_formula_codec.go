package cmd_context

import (
	"github.com/glycerine/g3/pkg/util/numeral"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
)

// EncodeOptimizationFormulas retains opt_context::to_fmls: hard constraints
// are followed by fresh Boolean objective-marker applications.
func (c *Context) EncodeOptimizationFormulas() (*opt.ObjectiveFormulaSet, error) {
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	hard := make([]ast.ASTHandle, len(c.assertions))
	for i := range c.assertions {
		hard[i] = c.assertions[i].expression
	}
	objectives := make([]opt.FormulaObjective, len(c.objectives))
	for i := range c.objectives {
		source := &c.objectives[i]
		if !source.soft {
			kind := opt.FormulaMinimize
			if source.maximize {
				kind = opt.FormulaMaximize
			}
			objectives[i] = opt.FormulaObjective{Kind: kind, Term: source.expression}
			continue
		}
		objective := opt.FormulaObjective{Kind: opt.FormulaMaxSAT, ID: source.id, SoftTerms: make([]ast.ASTHandle, len(source.softTerms))}
		objective.Weights = make([]numeral.Rational, len(source.softTerms))
		objectives[i] = objective
		for j := range source.softTerms {
			objective.SoftTerms[j] = source.softTerms[j].expression
			if objective.Weights[j], err = c.optClone(&source.softTerms[j].weight); err != nil {
				objective.Weights = objective.Weights[:j+1]
				objectives[i] = objective
				return nil, c.optDelObjectiveWeights(objectives, err)
			}
		}
	}
	set, err := opt.NewObjectiveFormulaSet(manager, hard, objectives)
	return set, c.optDelObjectiveWeights(objectives, err)
}

// optDelObjectiveWeights releases the formula objectives' weights (the
// codec keeps its own copies).
func (c *Context) optDelObjectiveWeights(objectives []opt.FormulaObjective, err error) error {
	for i := range objectives {
		err = c.optDelRats(objectives[i].Weights, err)
	}
	return err
}
