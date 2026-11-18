package opt

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrInvalidOptimizationValue = errors.New("g3: invalid optimization value")

// RestoreValue maps a MaxSAT penalty back to the source objective value.
// out may alias penalty.
func (o *DecomposedObjective) RestoreValue(q *numeral.RationalManager, penalty, out *numeral.Rational) error {
	if o == nil || penalty == nil || out == nil {
		return ErrInvalidOptimizationValue
	}
	if err := q.Copy(out, penalty); err != nil {
		return err
	}
	if o.Negate {
		if err := q.Neg(out); err != nil {
			return err
		}
	}
	return q.Add(out, &o.Offset, out)
}
