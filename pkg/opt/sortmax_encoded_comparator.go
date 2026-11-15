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
	notLeft, err := e.negate(left)
	if err != nil {
		return high, low, err
	}
	notRight, err := e.negate(right)
	if err != nil {
		return high, low, err
	}
	// sortmax reaches psort_nw with the zero-valued LE comparator mode in the
	// 4.7.1 source. These are the three cmp_le clauses for this concrete sort.
	if err = e.clause(notLeft, high); err != nil {
		return high, low, err
	}
	if err = e.clause(notRight, high); err != nil {
		return high, low, err
	}
	if err = e.clause(notLeft, notRight, low); err != nil {
		return high, low, err
	}
	return high, low, nil
}
