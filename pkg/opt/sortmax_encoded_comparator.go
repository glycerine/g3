package opt

// Source: src/opt/sortmax.cpp fresh/mk_max/mk_min comparator.

import "github.com/glycerine/g3/pkg/ast"

func (e *sortMaxEncoder) comparator(left, right ast.ASTHandle) (high, low ast.ASTHandle, err error) {
	high, err = e.fresh()
	if err != nil {
		return high, low, err
	}
	low, err = e.fresh()
	if err != nil {
		return high, low, err
	}
	if err = e.defineOr(high, left, right); err != nil {
		return high, low, err
	}
	if err = e.defineAnd(low, left, right); err != nil {
		return high, low, err
	}
	e.engine.stats.Comparators++
	return high, low, nil
}
