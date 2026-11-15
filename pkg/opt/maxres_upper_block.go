package opt

// Source: src/opt/maxres.cpp add_upper_bound_block.

import (
	"errors"
	"math/big"
)

// assertUpperBound is add_upper_bound_block. z3 4.7.1 bug kept out of g3:
// pb_util::mk_lt passes its own m_coeffs to mk_ge, whose normalize resets
// them first, so the blocker is a pbge with zero coefficients (false) and
// maxres stops at a non-optimal cost (pb_decl_plugin.cpp:165-178; fixed
// upstream; AUDIT.md opt_01-F4;
// test_vectors/z3_bug_maxres_upper_bound_block.smt2). g3 asserts a correct
// "cost < upper" blocker.
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
