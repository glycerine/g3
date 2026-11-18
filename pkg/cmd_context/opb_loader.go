package cmd_context

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
)

type OPBLoadSummary struct {
	Variables   int
	Constraints int
	Objectives  int
}

// LoadOPB builds and registers the statements of an OPB document in file
// order, as opb::parse does while it reads them: an objective is
// opt.add_objective(t, false), a constraint opt.add_hard_constraint(t).
func (c *Context) LoadOPB(document opt.OPBDocument) (OPBLoadSummary, error) {
	if c == nil {
		return OPBLoadSummary{}, fmt.Errorf("opb: nil command context")
	}
	manager, err := c.Manager()
	if err != nil {
		return OPBLoadSummary{}, err
	}
	variables := make(map[uint32]bool)
	summary := OPBLoadSummary{}
	for index := range document.Statements {
		statement := &document.Statements[index]
		if len(statement.Terms) == 0 {
			return OPBLoadSummary{}, fmt.Errorf("opb: statement %d is empty", index+1)
		}
		if statement.Objective {
			// parse_objective: t = mk_add(t, parse_term()) for min:,
			// mk_sub for max:.
			expression, e := c.makeOPBSum(statement.Terms, statement.MinimizeSyntax, variables)
			if e == nil {
				e = c.AddObjective(expression, false)
			}
			if e != nil {
				return OPBLoadSummary{}, fmt.Errorf("opb: statement %d: %w", index+1, e)
			}
			summary.Objectives++
			continue
		}
		left, e := c.makeOPBSum(statement.Terms, true, variables)
		if e != nil {
			return OPBLoadSummary{}, fmt.Errorf("opb: statement %d: %w", index+1, e)
		}
		right, e := opbNumeral(manager, statement.Bound)
		if e != nil {
			return OPBLoadSummary{}, e
		}
		var formula ast.ASTHandle
		switch statement.Relation {
		case opt.OPBGreaterEqual:
			formula, e = manager.MkArithmeticGE(left, right)
		case opt.OPBEqual:
			formula, e = manager.MkEq(left, right)
		case opt.OPBLessEqual:
			formula, e = manager.MkArithmeticLE(left, right)
		default:
			return OPBLoadSummary{}, fmt.Errorf("opb: statement %d has invalid relation", index+1)
		}
		if e == nil {
			e = c.Assert(formula, "")
		}
		if e != nil {
			return OPBLoadSummary{}, fmt.Errorf("opb: statement %d: %w", index+1, e)
		}
		summary.Constraints++
	}
	summary.Variables = len(variables)
	return summary, nil
}
