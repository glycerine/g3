package polynomial

// Source: src/math/polynomial/polynomial.cpp:2503-2638.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// GCDSimplifyRelation selects the source integer-content treatment. Equality
// divides every coefficient; strict relations preserve their integer solution
// set by rounding the constant term.
type GCDSimplifyRelation uint8

const (
	GCDSimplifyEquality GCDSimplifyRelation = iota
	GCDSimplifyLess
	GCDSimplifyGreater
)

// GCDSimplifyInto removes the greatest common divisor of the retained
// coefficients and publishes an independently owned result in out.
func (m *MultivariatePolynomialCore) GCDSimplifyInto(p Polynomial, relation GCDSimplifyRelation, out *PolynomialRef) (err error) {
	if out == nil || out.owner != m {
		return arena.ErrConfig
	}
	if relation > GCDSimplifyGreater {
		return arena.ErrConfig
	}
	zero, err := m.IsZero(p)
	if err != nil || zero {
		if err != nil {
			return err
		}
		return out.Assign(p)
	}
	z, err := m.IntegerManager()
	if err != nil {
		return err
	}
	g, coefficients, monomials, err := m.polynomialContent(z, p, relation)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, z.Del(&g))
		for i := range coefficients {
			err = errors.Join(err, z.Del(&coefficients[i]))
		}
	}()
	zero, err = z.IsZero(&g)
	if err != nil {
		return err
	}
	one, err := z.IsOne(&g)
	if err != nil {
		return err
	}
	if zero || one {
		return out.Assign(p)
	}
	for i := range coefficients {
		constant, cause := m.gcdSimplifyConstant(monomials[i])
		if cause != nil {
			return cause
		}
		if constant && relation != GCDSimplifyEquality {
			err = integerRoundedQuotient(z, &coefficients[i], &g, relation == GCDSimplifyGreater)
		} else {
			err = z.MachineDiv(&coefficients[i], &g, &coefficients[i])
		}
		if err != nil {
			return err
		}
	}
	return m.publishGCDSimplification(z, coefficients, monomials, out)
}

// integerRoundedQuotient sets a to floor(a/b) (or ceil(a/b) when ceil) for
// b > 0: the truncated quotient adjusted by the remainder's sign.
func integerRoundedQuotient(z *unumeral.IntegerManager, a, b *unumeral.Int, ceil bool) (err error) {
	var r unumeral.Int
	defer func() { err = errors.Join(err, z.Del(&r)) }()
	if err = z.Rem(a, b, &r); err != nil {
		return err
	}
	if err = z.MachineDiv(a, b, a); err != nil {
		return err
	}
	sign, err := z.Sign(&r)
	if err != nil {
		return err
	}
	var one unumeral.Int
	if err = z.SetInt64(&one, 1); err != nil {
		return err
	}
	if ceil && sign > 0 {
		return z.Add(a, &one, a)
	}
	if !ceil && sign < 0 {
		return z.Sub(a, &one, a)
	}
	return nil
}

// publishGCDSimplification builds the result from the borrowed values.
func (m *MultivariatePolynomialCore) publishGCDSimplification(z *unumeral.IntegerManager, values []unumeral.Int, monomials []Monomial, out *PolynomialRef) (err error) {
	coefficients := make([]unumeral.Int, len(values))
	defer func() {
		for i := range coefficients {
			err = errors.Join(err, z.Del(&coefficients[i]))
		}
	}()
	for i := range values {
		if err = z.Copy(&coefficients[i], &values[i]); err != nil {
			return err
		}
	}
	result, err := m.MkTerms(coefficients, monomials)
	if err != nil {
		return err
	}
	return out.Assign(result)
}
