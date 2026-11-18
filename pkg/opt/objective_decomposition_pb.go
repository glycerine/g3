package opt

// Source: src/opt/opt_context.cpp:843-891 and
// src/tactic/arith/lia2card_tactic.cpp:get_pb_sum.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic/arith"
)

// pseudoBooleanSum uses the same concrete extractor as the arithmetic tactic.
// Result terms acquire decomposer ownership before temporary owners are closed.
func (d *ObjectiveDecomposer) pseudoBooleanSum(root ast.ASTHandle) (result pbSum, ok bool, err error) {
	known := make(map[ast.ASTHandle]bool, len(d.zeroOne))
	for h := range d.zeroOne {
		known[h] = true
	}
	sum, ok, err := arith.ExtractPBSum(d.manager, root, known)
	if err != nil || !ok {
		return result, ok, err
	}
	defer func() { err = errors.Join(err, sum.Close()) }()
	// TODO bigC: arith.ExtractPBSum's coefficients are big.Rat.
	o := newRatOps(d.manager)
	result.offset = o.zero()
	o.fromBig(&result.offset, &sum.Constant)
	result.weights = make([]numeral.Rational, len(sum.Coefficients))
	for i := range sum.Coefficients {
		result.weights[i] = o.zero()
		o.fromBig(&result.weights[i], &sum.Coefficients[i])
	}
	if !o.ok() {
		return pbSum{}, false, errors.Join(o.err, result.release(o.q))
	}
	for _, term := range sum.Args {
		h, e := d.retain(term)
		if e != nil {
			return pbSum{}, false, errors.Join(e, result.release(o.q))
		}
		result.terms = append(result.terms, h)
	}
	return result, true, nil
}
