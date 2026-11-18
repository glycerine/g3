package qe

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// findMinMax is arith_plugin::find_min_max: bound (a rational of the
// manager's NumeralService().Manager) receives the extreme value.
func (c *arithmeticModelProjectCall) findMinMax(bounds []*ArithmeticBound, lower bool, bound *unumeral.Rational) (index uint32, found bool, resultErr error) {
	q := c.plugin.util.q()
	// rational num;
	num, abs := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&num), q.Del(&abs)) }()
	for i, b := range bounds {
		atom, cause := c.plugin.manager.RefGet(b.Atom)
		if cause != nil {
			return 0, false, cause
		}
		trueInModel, cause := c.truth(atom)
		if cause != nil {
			return 0, false, cause
		}
		if !trueInModel {
			continue
		}
		term, cause := c.plugin.manager.RefGet(b.Term)
		if cause != nil {
			return 0, false, cause
		}
		if cause = c.numeral(term, &num); cause != nil {
			return 0, false, cause
		}
		// num /= abs(coeff);
		if cause = errors.Join(q.Copy(&abs, &b.Coefficient), q.Abs(&abs)); cause != nil {
			return 0, false, cause
		}
		if cause = q.Div(&num, &abs, &num); cause != nil {
			return 0, false, cause
		}
		better := !found
		if found {
			c, cause := q.Compare(&num, bound)
			if cause != nil {
				return 0, false, cause
			}
			better = lower && c < 0 || !lower && c > 0
		}
		if better {
			if cause = q.Copy(bound, &num); cause != nil {
				return 0, false, cause
			}
			index, found = uint32(i), true
		}
	}
	return index, found, nil
}
