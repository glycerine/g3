package smtcore

// Source: src/smt/theory_str.cpp lower_bound/upper_bound and their retained
// theory_mi_arith get_lower/get_upper/to_expr calls in theory_arith_core.h.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// stringArithmeticBound reads the concrete mixed-arithmetic bound service.
// A candidate tableau value is never evidence of a bound. The native
// to_expr conversion refuses infinitesimals; the String caller additionally
// refuses nonintegral rational numerals, including bounds on Real terms.
func (t *SMTStringTheory) stringArithmeticBound(expression ast.ASTHandle, lower bool) (*big.Int, bool, error) {
	family, err := t.state.manager.ArithmeticFamilyID()
	if err != nil {
		return nil, false, err
	}
	node := t.state.context.enodes.get(expression)
	if port, ok := t.state.Theory(family).(*SMTTheoryArith); ok {
		// theory_str queries theory_mi_arith::get_lower/get_upper.
		if port.integerExt || node == nil {
			return nil, false, nil
		}
		get := port.getUpper
		if lower {
			get = port.getLower
		}
		h, known, err := get(node)
		if err != nil || !known {
			return nil, false, err
		}
		r, isNumeral := port.isNumeral(h)
		if !isNumeral || !r.IsInt() {
			return nil, false, nil
		}
		return new(big.Int).Set(r.Num()), true, nil
	}
	return nil, false, nil
}
