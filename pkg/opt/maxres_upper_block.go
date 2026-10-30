package opt

// Source: src/opt/maxres.cpp add_upper_bound_block.

import "math/big"

func (e *maxResEngine) assertUpperBound(satisfied *big.Rat, correctionSize int) error {
	if !e.upperBoundEligible(correctionSize) {
		return nil
	}
	block, err := e.solver.softBound(e.objective, satisfied, true)
	if err != nil {
		return err
	}
	block, err = e.owner.hold(block, nil)
	if err != nil {
		return err
	}
	if err = e.solver.backend.Assert(block); err != nil {
		return err
	}
	e.upperBoundActive = true
	e.stats.UpperBoundBlocks++
	return nil
}
