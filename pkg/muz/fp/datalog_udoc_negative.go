package fp

// Source: dl_compiler.cpp's variable_intersection mapping and negative-tail
// filter instructions, dispatched to the registered UDoc plugin.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type udocNegativeTail struct {
	declaration ast.ASTHandle
	variables   []uint32
}

func (c *datalogUDocContext) negativeRuleTails(rule *base.Rule) ([]udocNegativeTail, error) {
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil || positive == uninterpreted {
		return nil, err
	}
	if positive == 0 {
		return nil, ErrUDocDatalogFragment
	}
	bound, err := ast.NewASTUsedVars(c.manager)
	if err != nil {
		return nil, err
	}
	for index := 0; index < positive; index++ {
		tail, _, err := rule.Tail(index)
		if err != nil {
			return nil, err
		}
		if err = bound.Process(tail); err != nil {
			return nil, err
		}
	}
	result := make([]udocNegativeTail, 0, uninterpreted-positive)
	for index := positive; index < uninterpreted; index++ {
		literal, negative, err := rule.Tail(index)
		if err != nil || !negative {
			return nil, errors.Join(err, ErrUDocDatalogFragment)
		}
		declaration, err := c.manager.GetDecl(literal)
		if err != nil || c.result[declaration] == nil {
			return nil, errors.Join(err, ErrUDocDatalogFragment)
		}
		count, err := c.manager.GetNumArgs(literal)
		if err != nil {
			return nil, err
		}
		tail := udocNegativeTail{declaration: declaration}
		for column := uint32(0); column < count; column++ {
			argument, err := c.manager.GetArg(literal, column)
			if err != nil {
				return nil, err
			}
			kind, err := c.manager.GetKind(argument)
			if err != nil || kind != ast.ASTVar {
				return nil, errors.Join(err, ErrUDocDatalogFragment)
			}
			variable, err := c.manager.GetVarIndex(argument)
			if err != nil {
				return nil, err
			}
			sort, err := c.manager.GetSort(argument)
			if err != nil || bound.Contains(variable) != sort {
				return nil, errors.Join(err, fmt.Errorf("%w: unbound negative variable", ErrUDocDatalogFragment))
			}
			tail.variables = append(tail.variables, variable)
		}
		result = append(result, tail)
	}
	return result, nil
}

func (c *datalogUDocContext) filterNegativeTails(working *rel.TypedUDocRelation, rule *base.Rule, variables map[uint32]uint32) error {
	tails, err := c.negativeRuleTails(rule)
	if err != nil {
		return err
	}
	for _, tail := range tails {
		if err = c.checkpoint(); err != nil {
			return err
		}
		columns := make([]uint32, len(tail.variables))
		negativeColumns := make([]uint32, len(tail.variables))
		for index, variable := range tail.variables {
			column, found := variables[variable]
			if !found {
				return ErrUDocDatalogFragment
			}
			columns[index], negativeColumns[index] = column, uint32(index)
		}
		if err = working.FilterByNegation(c.result[tail.declaration], columns, negativeColumns); err != nil {
			return err
		}
	}
	return nil
}
