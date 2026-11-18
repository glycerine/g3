package polynomial

// Source: src/math/polynomial/polynomial.cpp:2503-2558 (later upstream
// gcd_simplify content computation over the mpz manager m()).

import (
	"errors"
	"math"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// polynomialContent returns g (the gcd of the coefficient magnitudes, one
// when a unit is seen) and owned copies of the coefficients. The caller
// deletes g and values through z.
func (m *MultivariatePolynomialCore) polynomialContent(z *unumeral.IntegerManager, p Polynomial, relation GCDSimplifyRelation) (g unumeral.Int, values []unumeral.Int, monomials []Monomial, err error) {
	info, err := m.Info(p)
	if err != nil {
		return g, nil, nil, err
	}
	values = make([]unumeral.Int, info.Size)
	monomials = make([]Monomial, info.Size)
	var magnitude unumeral.Int
	defer func() {
		err = errors.Join(err, z.Del(&magnitude))
		if err != nil {
			err = errors.Join(err, z.Del(&g))
			for i := range values {
				err = errors.Join(err, z.Del(&values[i]))
			}
			values = nil
		}
	}()
	slow := false
	for i := uint32(0); i < info.Size; i++ {
		coefficient, cause := m.Coefficient(p, i)
		if cause != nil {
			return g, values, nil, cause
		}
		if !coefficient.IsSmall() || coefficient.NativeScalar() == math.MinInt32 {
			slow = true
		}
		if err = z.Copy(&values[i], &coefficient); err != nil {
			return g, values, nil, err
		}
		monomials[i], cause = m.TermMonomial(p, i)
		if cause != nil {
			return g, values, nil, cause
		}
	}
	for i := range values {
		constant, cause := m.gcdSimplifyConstant(monomials[i])
		if cause != nil {
			return g, values, nil, cause
		}
		if err = z.Copy(&magnitude, &values[i]); err == nil {
			err = z.Abs(&magnitude)
		}
		if err != nil {
			return g, values, nil, err
		}
		unit, cause := z.IsOne(&magnitude)
		if cause != nil {
			return g, values, nil, cause
		}
		// The source fast path skips the strict-relation constant before
		// checking units. Its arbitrary-precision fallback checks units first.
		if slow && unit {
			return g, values, monomials, z.SetInt64(&g, 1)
		}
		if relation != GCDSimplifyEquality && constant {
			continue
		}
		if unit {
			return g, values, monomials, z.SetInt64(&g, 1)
		}
		if err = z.GCD(&g, &magnitude, &g); err != nil {
			return g, values, nil, err
		}
	}
	return g, values, monomials, nil
}

func (m *MultivariatePolynomialCore) gcdSimplifyConstant(term Monomial) (bool, error) {
	mm, err := m.MonomialManager()
	if err != nil {
		return false, err
	}
	info, err := mm.Info(term)
	return info.Size == 0, err
}
