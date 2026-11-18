package realclosure

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// integerContentValue is gcd_int_coeffs(value * a, mpz & g): g is an
// integer qm() rational; false when a coefficient is not an integer.
func (s *realValueStore) integerContentValue(value *realValue, gcd *numeral.Rational) (bool, error) {
	if value == nil {
		return true, nil
	}
	if !s.realOperand(value) {
		return false, nil
	}
	if !value.isRational {
		for _, coefficient := range value.functionNumerator.coefficients {
			ok, err := s.integerContentValue(coefficient, gcd)
			if err != nil || !ok {
				return false, err
			}
		}
		return true, nil
	}
	integer, err := s.numbers.IsInt(&value.rational)
	if err != nil || !integer {
		return false, err
	}
	zero, err := s.numbers.IsZero(gcd)
	if err != nil {
		return false, err
	}
	if zero {
		// qm().set(g, numerator); qm().abs(g)
		if err = s.numbers.Copy(gcd, &value.rational); err == nil {
			err = s.numbers.Abs(gcd)
		}
		return err == nil, err
	}
	if err = s.numbers.GCD(gcd, &value.rational, gcd); err != nil {
		return false, err
	}
	return true, nil
}

// normalizeIntegerContentNormalized is normalize_int_coeffs: divide every
// coefficient by the gcd of the integer coefficients when it is not one.
func (s *realValueStore) normalizeIntegerContentNormalized(values []*realValue) (err error) {
	gcd := numeral.NewRational()
	defer func() { err = errors.Join(err, s.numbers.Del(&gcd)) }()
	for _, value := range values {
		ok, e := s.integerContentValue(value, &gcd)
		if e != nil {
			return e
		}
		if !ok {
			return nil
		}
		one, e := s.numbers.IsOne(&gcd)
		if e != nil {
			return e
		}
		if one {
			return nil
		}
	}
	zero, err := s.numbers.IsZero(&gcd)
	if err != nil || zero {
		return err
	}
	coefficient, err := s.ownRationalPolynomial([]numeral.Rational{gcd})
	if err != nil || len(coefficient) != 1 {
		return errors.Join(err, arena.ErrInvalidSlot)
	}
	divisor := coefficient[0]
	for i, value := range values {
		if value == nil {
			continue
		}
		updated, cause := s.divideNormalizedValues(value, divisor)
		if cause != nil {
			return errors.Join(cause, s.release(divisor))
		}
		if cause = s.release(value); cause != nil {
			_ = s.release(updated)
			return errors.Join(cause, s.release(divisor))
		}
		values[i] = updated
	}
	return s.release(divisor)
}
