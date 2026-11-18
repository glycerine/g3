package arith

// Source: src/tactic/arith/fm_tactic.cpp:549-591.
import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// MakeInteger is mk_int: multiply as and c by the lcm of their
// denominators.
func (c *FMConstraint) MakeInteger() (resultErr error) {
	q := c.numerals
	// rational l = denominator(c);
	l, d := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&l), q.Del(&d)) }()
	if err := q.DenominatorRational(&c.Constant, &l); err != nil {
		return err
	}
	for i := range c.Coefficients {
		if err := q.DenominatorRational(&c.Coefficients[i], &d); err != nil {
			return err
		}
		if err := q.LCM(&l, &d, &l); err != nil {
			return err
		}
	}
	if l.IsOne() {
		return nil
	}
	if err := q.Mul(&c.Constant, &l, &c.Constant); err != nil {
		return err
	}
	for i := range c.Coefficients {
		if err := q.Mul(&c.Coefficients[i], &l, &c.Coefficients[i]); err != nil {
			return err
		}
	}
	return nil
}

// NormalizeCoefficients divides as and c by the gcd of their absolute
// values when they are integers.
func (c *FMConstraint) NormalizeCoefficients() (resultErr error) {
	q := c.numerals
	if len(c.Variables) == 0 {
		return nil
	}
	if !c.Constant.IsInt() {
		return nil
	}
	// rational g = abs(c);
	g, a := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&g), q.Del(&a)) }()
	if err := errors.Join(q.Copy(&g, &c.Constant), q.Abs(&g)); err != nil {
		return err
	}
	for i := range c.Coefficients {
		if !c.Coefficients[i].IsInt() {
			return nil
		}
		if err := errors.Join(q.Copy(&a, &c.Coefficients[i]), q.Abs(&a)); err != nil {
			return err
		}
		if err := q.GCD(&g, &a, &g); err != nil {
			return err
		}
		if g.IsOne() {
			return nil
		}
	}
	if g.IsZero() || g.IsOne() {
		return nil
	}
	if err := q.Div(&c.Constant, &g, &c.Constant); err != nil {
		return err
	}
	for i := range c.Coefficients {
		if err := q.Div(&c.Coefficients[i], &g, &c.Coefficients[i]); err != nil {
			return err
		}
	}
	return nil
}
