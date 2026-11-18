package realclosure

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// rationalPolynomialValues copies the rational coefficients of values
// (nullptr is zero) into a caller-owned qm() polynomial.
func (s *realValueStore) rationalPolynomialValues(values []*realValue) ([]numeral.Rational, error) {
	out := newQPoly(len(values))
	for i, value := range values {
		if !s.rationalOperand(value) {
			return nil, errors.Join(arena.ErrInvalidSlot, s.qPolyDel(out))
		}
		if value == nil {
			continue
		}
		if err := s.numbers.Copy(&out[i], &value.rational); err != nil {
			return nil, errors.Join(err, s.qPolyDel(out))
		}
	}
	return s.qPolyTrim(out)
}

// ownRationalPolynomial publishes copies of the coefficients of values as
// rational values (zero is nullptr); values stays owned by the caller.
func (s *realValueStore) ownRationalPolynomial(values []numeral.Rational) ([]*realValue, error) {
	out := make([]*realValue, len(values))
	for i := range values {
		zero, err := s.numbers.IsZero(&values[i])
		if err != nil {
			_ = s.releaseTemporaryCoefficients(out)
			return nil, err
		}
		if zero {
			continue
		}
		q := numeral.NewRational()
		if err = s.numbers.Copy(&q, &values[i]); err != nil {
			_ = s.releaseTemporaryCoefficients(out)
			return nil, errors.Join(err, s.numbers.Del(&q))
		}
		value, err := s.ownNonzeroRational(&q)
		if err != nil {
			_ = s.releaseTemporaryCoefficients(out)
			return nil, err
		}
		out[i] = value
	}
	return trimRealCoefficients(out), nil
}
