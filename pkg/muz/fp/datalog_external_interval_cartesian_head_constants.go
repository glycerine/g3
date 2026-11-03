package fp

// Source: dl_compiler.cpp make_add_constant_column and make_assembling_code.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (c *externalIntervalDatalogCartesianContinuation) closeProducts() error {
	if c.temporary != nil {
		if err := c.temporary.Close(); err != nil {
			return err
		}
		c.temporary = nil
	}
	if c.retiring != nil {
		if err := c.retiring.Close(); err != nil {
			return err
		}
		c.retiring = nil
	}
	if c.auxiliary != nil {
		if err := c.auxiliary.Close(); err != nil {
			return err
		}
		c.auxiliary = nil
	}
	return nil
}

func (r *externalIntervalDatalogRegisters) initializeCartesianHeadConstants(plan *externalIntervalDatalogCartesianRule) error {
	for _, constant := range plan.headConstants {
		if _, err := r.unarySingleton(plan.head, constant.sort, constant.value); err != nil {
			return err
		}
	}
	return nil
}

func (c *externalIntervalDatalogCartesianContinuation) appendHeadConstant(head ast.ASTHandle, constant externalIntervalCartesianHeadConstant) (bool, error) {
	singleton, err := c.registers.unarySingleton(head, constant.sort, constant.value)
	if err != nil {
		return false, err
	}
	// Even a nullary current is a real producer register. Its first constant
	// is appended with JOIN; only the singleton itself is borrowed from cache.
	joined, err := rel.JoinExternalIntervalProducts(c.temporary, singleton, nil, nil)
	if joined != nil {
		err = errors.Join(err, c.replaceTemporary(joined))
	}
	if err != nil {
		return false, err
	}
	return c.temporary.Empty()
}
