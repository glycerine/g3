package polynomial

import (
	"errors"
	"math"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/smallint"
)

// SetIsolatedSquareRoot constructs the unique root of X²-n in the open
// interval (lower, lower+1). Integer endpoints certify both isolation and
// irreducibility over the rationals. Perfect squares have no such interval.
func (m *AlgebraicRootCore) SetIsolatedSquareRoot(dst *Algebraic, n, lower int32) (err error) {
	return m.SetIsolatedSquareRootFraction(dst, n, 1, lower)
}

// SetIsolatedSquareRootFraction installs the root of denominator*X²-numerator
// in (lower, lower+1). The polynomial must be primitive and irreducible.
func (m *AlgebraicRootCore) SetIsolatedSquareRootFraction(dst *Algebraic, numerator, denominator, lower int32) (err error) {
	if _, err = m.location(dst); err != nil {
		return err
	}
	if numerator <= 0 || denominator <= 0 {
		return ErrAlgebraicAssertion
	}
	lo, hi := int64(lower), int64(lower)+1
	left, right := lo*lo, hi*hi
	// Compare the squared endpoints with the fraction by integer division.
	// Multiplying a caller-supplied squared endpoint by denominator can overflow.
	n, d := int64(numerator), int64(denominator)
	if min(left, right) > (n-1)/d || max(left, right) <= n/d {
		return ErrAlgebraicAssertion
	}
	if denominator > 1 {
		if smallint.GCD(smallint.New(numerator), smallint.New(denominator)) != 1 {
			return ErrAlgebraicAssertion
		}
		p, q := int64(math.Sqrt(float64(n))), int64(math.Sqrt(float64(d)))
		if p*p == n && q*q == d {
			return ErrAlgebraicAssertion
		}
	}
	pm := m.config.Polynomials
	p, err := pm.NewPolynomial()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, pm.DelPolynomial(&p)) }()
	coefficients := [...]unumeral.Int{unumeral.NewInt(-numerator), unumeral.NewInt(0), unumeral.NewInt(denominator)}
	if err = pm.SetInts(p, coefficients[:]); err != nil {
		return err
	}
	view, err := pm.View(p, 0, 3)
	if err != nil {
		return err
	}
	a, b := unumeral.NewBinaryInt(lower), unumeral.NewBinaryInt(int32(hi))
	return m.SetRoot(dst, view, &a, &b, true)
}
