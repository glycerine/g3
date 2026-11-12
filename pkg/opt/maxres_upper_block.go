package opt

// Source: src/opt/maxres.cpp add_upper_bound_block.

import (
	"errors"
	"math/big"
)

func (e *maxResEngine) assertUpperBound(satisfied *big.Rat) error {
	if !e.upperBoundEligible() {
		return nil
	}
	block, err := e.solver.softBound(e.objective, satisfied, true)
	if err != nil {
		return err
	}
	// expr_ref fml is add_upper_bound_block's local.
	fml := &maxResLocal{m: e.solver.manager}
	if block, err = fml.set(block, nil); err == nil {
		err = e.solver.solver().Assert(block)
	}
	if err = errors.Join(err, fml.release()); err != nil {
		return err
	}
	e.stats.UpperBoundBlocks++
	return nil
}
