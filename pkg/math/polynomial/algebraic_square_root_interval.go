package polynomial

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// SetIsolatedSquareRoot constructs the unique root of X²-n in the open
// interval (lower, lower+1). Integer endpoints certify both isolation and
// irreducibility over the rationals. Perfect squares have no such interval.
func (m *AlgebraicRootCore) SetIsolatedSquareRoot(dst *Algebraic, n, lower int32) (err error) {
	if _, err = m.location(dst); err != nil {
		return err
	}
	lo, hi := int64(lower), int64(lower)+1
	left, right := lo*lo, hi*hi
	if !(min(left, right) < int64(n) && int64(n) < max(left, right)) {
		return ErrAlgebraicAssertion
	}
	pm := m.config.Polynomials
	p, err := pm.NewPolynomial()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, pm.DelPolynomial(&p)) }()
	coefficients := [...]unumeral.Int{unumeral.NewInt(-n), unumeral.NewInt(0), unumeral.NewInt(1)}
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
