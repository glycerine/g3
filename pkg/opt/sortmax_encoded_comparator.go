package opt

// Source: src/opt/sortmax.cpp fresh/mk_max/mk_min comparator.

import "github.com/glycerine/g3/pkg/ast"

func (e *sortMaxEncoder) comparator(left, right ast.ASTHandle) (high, low ast.ASTHandle, err error) {
	if left == right {
		// psort_nw::mk_max/mk_min return a when a == b; cmp() still emits
		// its clauses over y1 = y2 = a.
		high, low = left, left
	} else {
		high, err = e.engine.owner.hold(e.engine.solver.manager.MkOr(left, right))
		if err != nil {
			return high, low, err
		}
		low, err = e.engine.owner.hold(e.engine.solver.manager.MkAnd(left, right))
		if err != nil {
			return high, low, err
		}
		e.engine.stats.Comparators++
	}
	// cmp_ge: y2 -> x1, y2 -> x2, y1 -> x1 or x2. psort_nw's constructor
	// leaves m_t uninitialized in 4.7.1 (undefined behavior; sortmax never
	// calls ge/le/eq to set it). Fork fix #A initializes it to GE, as
	// upstream z3 does, so these are the cmp_ge clauses.
	notLow, err := e.negate(low)
	if err != nil {
		return high, low, err
	}
	if err = e.clause(notLow, left); err != nil {
		return high, low, err
	}
	notLow, err = e.negate(low)
	if err != nil {
		return high, low, err
	}
	if err = e.clause(notLow, right); err != nil {
		return high, low, err
	}
	notHigh, err := e.negate(high)
	if err != nil {
		return high, low, err
	}
	if err = e.clause(notHigh, left, right); err != nil {
		return high, low, err
	}
	return high, low, nil
}
