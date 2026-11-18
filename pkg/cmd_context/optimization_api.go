package cmd_context

// Source: src/opt/opt_context.cpp objective enumeration/value access;
// src/api/api_opt.cpp objective handles and get-objectives.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
)

// OptimizationSoftTerm's Weight is a rational of the AST manager's
// NumeralService().Manager owned by the OptimizationObjective.
type OptimizationSoftTerm struct {
	Expression ast.ASTHandle
	Weight     numeral.Rational
}

type OptimizationObjective struct {
	Expression ast.ASTHandle
	ID         string
	SoftTerms  []OptimizationSoftTerm
	Maximize   bool
	Soft       bool
	BitVector  bool
}

// Release destroys the soft terms' weights (q is the AST manager's
// NumeralService().Manager).
func (o *OptimizationObjective) Release(q *numeral.RationalManager) (err error) {
	for i := range o.SoftTerms {
		err = errors.Join(err, q.Del(&o.SoftTerms[i].Weight))
	}
	return err
}

// OptimizationObjectiveValue's SoftCost, Value and ModelValue are rationals
// of the AST manager's NumeralService().Manager owned by the value (Release).
type OptimizationObjectiveValue struct {
	Value, ModelValue simplex.Infinitesimal
	SoftCost          numeral.Rational
	Available         bool
	Unbounded         bool
	ModelUnbounded    bool
	Interval          bool
	OpenBounds        bool
}

// Release destroys SoftCost, Value and ModelValue (q is the AST manager's
// NumeralService().Manager).
func (v *OptimizationObjectiveValue) Release(q *numeral.RationalManager) error {
	return errors.Join(q.Del(&v.SoftCost), v.Value.Del(q), v.ModelValue.Del(q))
}

func (c *Context) ObjectiveCount() (uint32, error) {
	if err := c.valid(); err != nil {
		return 0, err
	}
	return uint32(len(c.objectives)), nil
}

// Objective's result owns its weights (Release).
func (c *Context) Objective(index uint32) (OptimizationObjective, error) {
	if err := c.valid(); err != nil {
		return OptimizationObjective{}, err
	}
	if uint64(index) >= uint64(len(c.objectives)) {
		return OptimizationObjective{}, ast.ErrASTDomain
	}
	source := &c.objectives[index]
	result := OptimizationObjective{Expression: source.expression, ID: source.id, Maximize: source.maximize, Soft: source.soft, BitVector: source.bitVector, SoftTerms: make([]OptimizationSoftTerm, len(source.softTerms))}
	for i := range source.softTerms {
		result.SoftTerms[i].Expression = source.softTerms[i].expression
		result.SoftTerms[i].Weight = numeral.NewRational()
		if err := c.optQ().Copy(&result.SoftTerms[i].Weight, &source.softTerms[i].weight); err != nil {
			return OptimizationObjective{}, errors.Join(err, result.Release(c.optQ()))
		}
	}
	return result, nil
}

// ObjectiveValue's result owns its rationals (Release).
func (c *Context) ObjectiveValue(index uint32) (OptimizationObjectiveValue, error) {
	if err := c.valid(); err != nil {
		return OptimizationObjectiveValue{}, err
	}
	if uint64(index) >= uint64(len(c.objectives)) {
		return OptimizationObjectiveValue{}, ast.ErrASTDomain
	}
	source := &c.objectives[index]
	result := OptimizationObjectiveValue{Available: source.valueReady, Unbounded: source.value.Unbounded, ModelUnbounded: source.modelValue.Unbounded, Interval: source.interval, OpenBounds: source.openBounds}
	q := c.optQ()
	result.Value, result.ModelValue, result.SoftCost = simplex.ZeroInfinitesimal(), simplex.ZeroInfinitesimal(), numeral.NewRational()
	if err := errors.Join(result.Value.Set(q, &source.value.Finite), result.ModelValue.Set(q, &source.modelValue.Finite), q.Copy(&result.SoftCost, &source.softValue)); err != nil {
		return OptimizationObjectiveValue{}, errors.Join(err, result.Release(c.optQ()))
	}
	return result, nil
}

// AddSoftConstraintIndexed borrows weight (a rational of the AST manager's
// NumeralService().Manager).
func (c *Context) AddSoftConstraintIndexed(expression ast.ASTHandle, weight *numeral.Rational, id string) (uint32, error) {
	if err := c.AddSoftConstraint(expression, weight, id); err != nil {
		return 0, err
	}
	return c.softObjectiveIndexOf(softObjectiveKey{text: id})
}

// AddSoftConstraintEmptySymbolIndexed is AddSoftConstraintIndexed with the
// non-null symbol("") as id.
func (c *Context) AddSoftConstraintEmptySymbolIndexed(expression ast.ASTHandle, weight *numeral.Rational) (uint32, error) {
	if err := c.AddSoftConstraintEmptySymbol(expression, weight); err != nil {
		return 0, err
	}
	return c.softObjectiveIndexOf(softObjectiveKey{emptySymbol: true})
}

func (c *Context) softObjectiveIndexOf(key softObjectiveKey) (uint32, error) {
	index, found := c.softObjectiveIndex[key]
	if !found {
		return 0, ast.ErrASTDomain
	}
	return uint32(index), nil
}

// WriteOptimizationReplaySMT2 extends the ordinary command-context replay
// with the active optimization objectives.  Objective terms are emitted after
// the live assertion stack: this is the same current optimize problem, and it
// keeps the declaration collection performed by WriteReplaySMT2 authoritative.
func (c *Context) WriteOptimizationReplaySMT2(out io.Writer) error {
	if err := c.WriteReplaySMT2(out); err != nil {
		return err
	}
	for i := range c.objectives {
		objective := &c.objectives[i]
		if objective.soft {
			for j := range objective.softTerms {
				term := &objective.softTerms[j]
				if _, err := io.WriteString(out, "(assert-soft "); err != nil {
					return err
				}
				if err := c.WriteExpression(out, term.expression); err != nil {
					return err
				}
				if _, err := io.WriteString(out, " :weight "); err != nil {
					return err
				}
				if err := c.writeSoftObjectiveValue(out, &term.weight); err != nil {
					return err
				}
				if objective.id != "" {
					symbol, err := c.manager.Symbols.Intern(objective.id)
					if err != nil {
						return err
					}
					name, err := c.smt2SymbolName(symbol)
					if err != nil {
						return err
					}
					if _, err = io.WriteString(out, " :id "+name); err != nil {
						return err
					}
				}
				if _, err := io.WriteString(out, ")\n"); err != nil {
					return err
				}
			}
			continue
		}
		command := "minimize"
		if objective.maximize {
			command = "maximize"
		}
		if _, err := io.WriteString(out, "("+command+" "); err != nil {
			return err
		}
		if err := c.WriteExpression(out, objective.expression); err != nil {
			return err
		}
		if _, err := io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	return nil
}
