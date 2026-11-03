package fp

// Source: dl_compiler.cpp compile_rule adds full columns for free variables
// in the interpreted conjunction before constructing its column substitution.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (c *datalogUDocContext) addUDocConditionColumns(working **rel.TypedUDocRelation, rule *base.Rule, first, total int, variables map[uint32]uint32) error {
	if first == total {
		return nil
	}
	if working == nil || *working == nil {
		return ErrUDocDatalogFragment
	}
	used, err := ast.NewASTUsedVars(c.manager)
	if err != nil {
		return err
	}
	for i := first; i < total; i++ {
		condition, negated, cause := rule.Tail(i)
		if cause != nil {
			return cause
		}
		if negated {
			return ErrUDocDatalogFragment
		}
		if cause = used.Process(condition); cause != nil {
			return cause
		}
	}
	// Validate every missing column before changing the working relation.
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		sort := used.Contains(variable)
		if sort.IsNull() {
			continue
		}
		if _, found := variables[variable]; found {
			continue
		}
		supported, cause := c.factory.CanHandleSignature([]ast.ASTHandle{sort})
		if cause != nil {
			return cause
		}
		if !supported {
			return fmt.Errorf("%w: unsupported interpreted variable sort", ErrUDocDatalogFragment)
		}
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		sort := used.Contains(variable)
		if sort.IsNull() {
			continue
		}
		if _, found := variables[variable]; found {
			continue
		}
		if err = c.checkpoint(); err != nil {
			return err
		}
		column, cause := c.factory.NewFull([]ast.ASTHandle{sort})
		if cause != nil {
			return cause
		}
		joined, cause := (*working).Join(column, nil, nil)
		if cause != nil {
			return errors.Join(cause, column.Close())
		}
		prior := *working
		index := uint32(len(prior.Sorts()))
		*working = joined
		variables[variable] = index
		if cause = errors.Join(prior.Close(), column.Close()); cause != nil {
			return cause
		}
	}
	return nil
}
